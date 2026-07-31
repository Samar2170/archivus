package config

import (
	archivus_constants "archivus/internal/constants"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// debugMode is overridden at build time by release builds:
//
//	go build -ldflags "-X archivus/internal/config.debugMode=false"
//
// The linker can only set strings, hence the indirection through DEBUG.
var debugMode = "true"

// DEBUG keeps config and user data in the working directory instead of the
// user's home directory. It must be false in production.
var DEBUG = debugMode == "true"

type Configuration struct {
	DefaultWriteAccess bool   `yaml:"default_write_access"`
	AllowUserDrive     bool   `yaml:"allow_user_drive"`
	LogsDir            string `yaml:"logs_dir"`
	SecretKey          string `yaml:"secret_key"`
	ArchivusHome       string `yaml:"base_dir"`
	ServerSalt         string `yaml:"server_salt"`
	BackendProxyUrl    string `yaml:"backend_proxy_url"`

	ThumbnailDir string `yaml:"thumbnail_dir"`

	// S3Enabled is derived from the server mode passed to Init, never from the
	// config file — persisting it would let a stale `s3_enabled: true` survive a
	// switch back to home mode and leave S3Cfg nil at the point of use.
	S3Enabled bool `yaml:"-"`
}

var (
	Config         *Configuration
	S3Cfg          *S3Config
	ProjectBaseDir string
	UsersDir       string
)

func (c *Configuration) String() string {
	return fmt.Sprintf(
		"ArchivusHome:       %s\nLogsDir:            %s\nBackendProxyUrl:    %s\nDefaultWriteAccess: %v\nAllowUserDrive:     %v\nS3Enabled:          %v\nSecretKey:          [redacted]\nServerSalt:         [redacted]",
		c.ArchivusHome, c.LogsDir, c.BackendProxyUrl,
		c.DefaultWriteAccess, c.AllowUserDrive, c.S3Enabled,
	)
}

// Init resolves the configuration and publishes it on Config, ProjectBaseDir
// and UsersDir. Must be called before any other package that reads Config.
//
// Values are layered lowest to highest precedence:
//
//	defaults()  -> config.yaml -> ARCHIVUS_* env -> serverMode
//
// Only the first two layers are ever written back to disk, so an env override
// applies to the current run without being baked into the file.
func Init(serverMode string) error {
	home, err := baseDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	root, err := settingsDir(home)
	if err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg := defaults(home)

	configPath := filepath.Join(root, archivus_constants.ConfigFileName)
	existed, err := applyFile(cfg, configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	generated, err := fillSecrets(cfg)
	if err != nil {
		return fmt.Errorf("build default config: %w", err)
	}

	// Persist before env overrides are layered on, so the file records the
	// durable configuration only.
	if !existed || generated {
		if err := save(cfg, configPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	if err := applyEnv(cfg); err != nil {
		return err
	}
	cfg.S3Enabled = serverMode == "biz"

	// Derived paths are resolved once, after every explicit source has had its
	// say, so that overriding only the base dir still moves everything under it.
	cfg.derive(root)

	if err := cfg.ensureDirs(root); err != nil {
		return err
	}

	if cfg.S3Enabled {
		S3Cfg, err = LoadS3Config(s3ConfigPaths(home, root))
		if err != nil {
			return fmt.Errorf("load s3 config: %w", err)
		}
	}

	Config = cfg
	ProjectBaseDir = root
	UsersDir = filepath.Join(cfg.ArchivusHome, usersDirName)
	return nil
}

const usersDirName = "users"

// defaults returns the baseline configuration. It performs no I/O and leaves
// every derived path empty — derive fills those in once the file and env layers
// have been applied.
func defaults(home string) *Configuration {
	return &Configuration{
		DefaultWriteAccess: false,
		AllowUserDrive:     true,
		ArchivusHome:       filepath.Join(home, "archivus"),
	}
}

// applyFile unmarshals path over cfg, leaving fields absent from the file at
// their default. It reports whether the file existed; a missing file is not an
// error.
func applyFile(cfg *Configuration, path string) (bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return true, err
	}
	return true, nil
}

// applyEnv layers ARCHIVUS_* overrides on top of cfg. These are runtime-only
// and are never written back to the config file.
func applyEnv(cfg *Configuration) error {
	envString("ARCHIVUS_HOME", &cfg.ArchivusHome)
	envString("ARCHIVUS_LOGS_DIR", &cfg.LogsDir)
	envString("ARCHIVUS_THUMBNAIL_DIR", &cfg.ThumbnailDir)
	envString("ARCHIVUS_SECRET_KEY", &cfg.SecretKey)
	envString("ARCHIVUS_SERVER_SALT", &cfg.ServerSalt)
	envString("ARCHIVUS_BACKEND_PROXY_URL", &cfg.BackendProxyUrl)
	if err := envBool("ARCHIVUS_DEFAULT_WRITE_ACCESS", &cfg.DefaultWriteAccess); err != nil {
		return err
	}
	return envBool("ARCHIVUS_ALLOW_USER_DRIVE", &cfg.AllowUserDrive)
}

// fillSecrets generates any secret left empty by the earlier layers and reports
// whether it had to. Existing values are preserved — regenerating them would
// invalidate every issued token on restart.
func fillSecrets(cfg *Configuration) (bool, error) {
	var generated bool
	for _, s := range []struct {
		field  *string
		length int
	}{
		{&cfg.SecretKey, 32},
		{&cfg.ServerSalt, 16},
	} {
		if *s.field != "" {
			continue
		}
		v, err := generateRandomAlphaNumericString(s.length)
		if err != nil {
			return generated, err
		}
		*s.field = v
		generated = true
	}
	return generated, nil
}

// derive fills in the paths that default to a location under ArchivusHome or
// the settings dir. An explicit value from any layer wins.
func (c *Configuration) derive(root string) {
	if c.LogsDir == "" {
		c.LogsDir = filepath.Join(root, "logs")
	}
	if c.ThumbnailDir == "" {
		c.ThumbnailDir = filepath.Join(c.ArchivusHome, archivus_constants.ThumbnailDirName)
	}
}

// ensureDirs creates every directory the running server expects to exist.
func (c *Configuration) ensureDirs(root string) error {
	for _, d := range []struct{ name, path string }{
		{"config", root},
		{"archivus home", c.ArchivusHome},
		{"logs", c.LogsDir},
		{"thumbnail", c.ThumbnailDir},
		{"users", filepath.Join(c.ArchivusHome, usersDirName)},
	} {
		if err := os.MkdirAll(d.path, os.ModePerm); err != nil {
			return fmt.Errorf("create %s dir: %w", d.name, err)
		}
	}
	return nil
}

// baseDir is the root under which the settings dir and default storage live:
// the working directory in debug builds, the user's home otherwise.
func baseDir() (string, error) {
	if DEBUG {
		return os.Getwd()
	}
	return os.UserHomeDir()
}

// settingsDir returns the .archivus directory, honouring ARCHIVUS_CONFIG_DIR,
// and makes sure it exists.
func settingsDir(home string) (string, error) {
	root := os.Getenv("ARCHIVUS_CONFIG_DIR")
	if root == "" {
		root = filepath.Join(home, archivus_constants.SettingsDir)
	}
	if err := os.MkdirAll(root, os.ModePerm); err != nil {
		return "", err
	}
	return root, nil
}

func save(cfg *Configuration, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
