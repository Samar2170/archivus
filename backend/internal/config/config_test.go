package config

import (
	archivus_constants "archivus/internal/constants"
	"os"
	"path/filepath"
	"testing"
)

// withTempRoot isolates a test from the developer's real environment: it moves
// into a temp working directory (debug builds root storage there), points
// ARCHIVUS_CONFIG_DIR at a settings dir inside it, and resets the package
// globals afterwards.
func withTempRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	root := filepath.Join(dir, ".archivus")
	t.Setenv("ARCHIVUS_CONFIG_DIR", root)
	t.Cleanup(func() {
		Config, S3Cfg, ProjectBaseDir, UsersDir = nil, nil, "", ""
	})
	return root
}

func TestInitWritesDefaultConfig(t *testing.T) {
	root := withTempRoot(t)

	if err := Init("home"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, archivus_constants.ConfigFileName)); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if len(Config.SecretKey) != 32 || len(Config.ServerSalt) != 16 {
		t.Errorf("secrets not generated: key=%d salt=%d", len(Config.SecretKey), len(Config.ServerSalt))
	}
	if !Config.AllowUserDrive || Config.DefaultWriteAccess {
		t.Errorf("unexpected defaults: %+v", Config)
	}
	for _, dir := range []string{Config.ArchivusHome, Config.LogsDir, Config.ThumbnailDir, UsersDir} {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Errorf("directory %q not created: %v", dir, err)
		}
	}
}

func TestInitPreservesSecretsAcrossRuns(t *testing.T) {
	withTempRoot(t)

	if err := Init("home"); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	key, salt := Config.SecretKey, Config.ServerSalt

	if err := Init("home"); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if Config.SecretKey != key || Config.ServerSalt != salt {
		t.Error("secrets regenerated on restart; issued tokens would be invalidated")
	}
}

// A config file predating a field must not leave that field empty — the
// defaults and derive layers fill the gap.
func TestInitBackfillsMissingFields(t *testing.T) {
	root := withTempRoot(t)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	partial := "base_dir: " + filepath.Join(t.TempDir(), "storage") + "\nsecret_key: abc\n"
	if err := os.WriteFile(filepath.Join(root, archivus_constants.ConfigFileName), []byte(partial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Init("home"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Config.LogsDir == "" || Config.ThumbnailDir == "" {
		t.Errorf("derived paths left empty: %+v", Config)
	}
	if !Config.AllowUserDrive {
		t.Error("AllowUserDrive default lost when absent from file")
	}
	if Config.SecretKey != "abc" {
		t.Errorf("file value overridden: %q", Config.SecretKey)
	}
}

// Derived paths must follow an overridden base dir rather than the default one.
func TestDerivedPathsFollowBaseDir(t *testing.T) {
	withTempRoot(t)
	home := filepath.Join(t.TempDir(), "storage")
	t.Setenv("ARCHIVUS_HOME", home)

	if err := Init("home"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := filepath.Join(home, archivus_constants.ThumbnailDirName)
	if Config.ThumbnailDir != want {
		t.Errorf("ThumbnailDir = %q, want %q", Config.ThumbnailDir, want)
	}
	if UsersDir != filepath.Join(home, usersDirName) {
		t.Errorf("UsersDir = %q, want under %q", UsersDir, home)
	}
}

func TestEnvOverridesFileButIsNotPersisted(t *testing.T) {
	root := withTempRoot(t)
	if err := Init("home"); err != nil {
		t.Fatalf("seed Init: %v", err)
	}
	onDisk := Config.ArchivusHome

	override := filepath.Join(t.TempDir(), "override")
	t.Setenv("ARCHIVUS_HOME", override)
	t.Setenv("ARCHIVUS_DEFAULT_WRITE_ACCESS", "true")
	if err := Init("home"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Config.ArchivusHome != override {
		t.Errorf("env override ignored: %q", Config.ArchivusHome)
	}
	if !Config.DefaultWriteAccess {
		t.Error("ARCHIVUS_DEFAULT_WRITE_ACCESS not applied")
	}

	// The file must still hold the durable value, not the env one.
	saved := defaults("unused")
	if _, err := applyFile(saved, filepath.Join(root, archivus_constants.ConfigFileName)); err != nil {
		t.Fatal(err)
	}
	if saved.ArchivusHome != onDisk {
		t.Errorf("env value leaked into config file: %q", saved.ArchivusHome)
	}
	if saved.DefaultWriteAccess {
		t.Error("env bool leaked into config file")
	}
}

func TestEnvBoolRejectsGarbage(t *testing.T) {
	withTempRoot(t)
	t.Setenv("ARCHIVUS_ALLOW_USER_DRIVE", "yesplease")
	if err := Init("home"); err == nil {
		t.Fatal("expected an error for an unparseable boolean")
	}
}

// Home mode must not inherit s3_enabled from a config written in biz mode —
// doing so used to leave S3Cfg nil at the point of use.
func TestModeDeterminesS3Enabled(t *testing.T) {
	root := withTempRoot(t)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, archivus_constants.ConfigFileName),
		[]byte("s3_enabled: true\nsecret_key: abc\n"), 0644,
	); err != nil {
		t.Fatal(err)
	}

	if err := Init("home"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Config.S3Enabled {
		t.Error("stale s3_enabled from config file survived home mode")
	}
	if S3Cfg != nil {
		t.Error("S3Cfg loaded in home mode")
	}
}
