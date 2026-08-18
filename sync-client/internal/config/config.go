// Package config manages the archivus-sync client's on-disk configuration:
// the server URL, auth token, and the list of tracked folders. Everything lives
// under a per-user directory (~/.archivus-sync by default, overridable with
// ARCHIVUS_SYNC_HOME) so the client is self-contained on a desktop.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DirName is the client's settings directory under the user's home.
	DirName = ".archivus-sync"
	// FileName is the legacy root-level config file within DirName.
	FileName = "config.json"
	// ProfilesDirName stores per-server profile directories.
	ProfilesDirName = "profiles"
	// ActiveServerFileName stores the currently selected server URL.
	ActiveServerFileName = "active_server"
	// StateFileName is the persisted sync state file name.
	StateFileName = "state.json"
	// HomeEnv overrides the settings directory location (useful for tests and
	// for running the client as a service account).
	HomeEnv = "ARCHIVUS_SYNC_HOME"
)

// TrackedFolder is a local directory that the client keeps synced up into a
// drive folder on each sync run.
type TrackedFolder struct {
	LocalPath string `json:"localPath"` // absolute path of the folder to watch
	DriveID   string `json:"driveId"`   // destination drive id
	DestRel   string `json:"destRel"`   // destination folder within the drive (may be empty for root)
}

// Config is the full client configuration persisted as JSON.
type Config struct {
	ServerURL string          `json:"serverUrl"`
	Token     string          `json:"token"`
	Username  string          `json:"username"`
	Tracked   []TrackedFolder `json:"tracked"`
}

// Dir returns the client settings directory, creating it if needed.
func Dir() (string, error) {
	base := os.Getenv(HomeEnv)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, DirName)
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %q: %w", base, err)
	}
	return base, nil
}

// Path returns the absolute path of the config file.
func Path() (string, error) {
	serverURL, err := ActiveServerURL()
	if err != nil {
		return "", err
	}
	if serverURL != "" {
		return PathForServer(serverURL)
	}
	return legacyConfigPath()
}

func legacyConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// PathForServer returns the config path for a specific server profile.
func PathForServer(serverURL string) (string, error) {
	dir, err := profileDir(serverURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// StatePathForServer returns the state.json path for a specific server profile.
// If serverURL is empty, it points to the legacy root-level state path.
func StatePathForServer(serverURL string) (string, error) {
	serverURL = normalizeServerURL(serverURL)
	if serverURL == "" {
		dir, err := Dir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, StateFileName), nil
	}
	dir, err := profileDir(serverURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StateFileName), nil
}

// ActiveServerURL returns the current server profile selection.
func ActiveServerURL() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, ActiveServerFileName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read active server: %w", err)
	}
	return normalizeServerURL(string(data)), nil
}

func setActiveServerURL(serverURL string) error {
	serverURL = normalizeServerURL(serverURL)
	if serverURL == "" {
		return nil
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, ActiveServerFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(serverURL+"\n"), 0o600); err != nil {
		return fmt.Errorf("write active server %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace active server %q: %w", path, err)
	}
	return nil
}

// Load reads the config file. A missing file yields an empty Config so first-run
// commands can populate and save it.
func Load() (*Config, error) {
	serverURL, err := ActiveServerURL()
	if err != nil {
		return nil, err
	}
	if serverURL != "" {
		return LoadForServer(serverURL)
	}

	path, err := legacyConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	c.ServerURL = normalizeServerURL(c.ServerURL)
	return &c, nil
}

// LoadForServer reads the config for the given server URL profile.
func LoadForServer(serverURL string) (*Config, error) {
	serverURL = normalizeServerURL(serverURL)
	if serverURL == "" {
		return Load()
	}

	path, err := PathForServer(serverURL)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{ServerURL: serverURL}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if c.ServerURL == "" {
		c.ServerURL = serverURL
	}
	c.ServerURL = normalizeServerURL(c.ServerURL)
	return &c, nil
}

// Save writes the config file atomically with owner-only permissions (it holds a
// bearer token).
func (c *Config) Save() error {
	c.ServerURL = normalizeServerURL(c.ServerURL)

	var (
		path string
		err  error
	)
	if c.ServerURL == "" {
		path, err = legacyConfigPath()
	} else {
		path, err = PathForServer(c.ServerURL)
	}
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace config %q: %w", path, err)
	}
	if err := setActiveServerURL(c.ServerURL); err != nil {
		return err
	}
	return nil
}

// LoggedIn reports whether a server URL and token are configured.
func (c *Config) LoggedIn() bool {
	return c.ServerURL != "" && c.Token != ""
}

func normalizeServerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	return raw
}

func profileDir(serverURL string) (string, error) {
	serverURL = normalizeServerURL(serverURL)
	if serverURL == "" {
		return "", fmt.Errorf("server URL is required")
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	profilesDir := filepath.Join(dir, ProfilesDirName)
	if err := os.MkdirAll(profilesDir, 0o700); err != nil {
		return "", fmt.Errorf("create profiles dir %q: %w", profilesDir, err)
	}
	hash := sha256.Sum256([]byte(serverURL))
	key := hex.EncodeToString(hash[:])
	p := filepath.Join(profilesDir, key)
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", fmt.Errorf("create profile dir %q: %w", p, err)
	}
	return p, nil
}

// AddTracked adds or replaces (by LocalPath) a tracked folder.
func (c *Config) AddTracked(tf TrackedFolder) {
	for i, existing := range c.Tracked {
		if existing.LocalPath == tf.LocalPath {
			c.Tracked[i] = tf
			return
		}
	}
	c.Tracked = append(c.Tracked, tf)
}

// RemoveTracked removes a tracked folder by LocalPath, reporting whether one was
// found.
func (c *Config) RemoveTracked(localPath string) bool {
	for i, existing := range c.Tracked {
		if existing.LocalPath == localPath {
			c.Tracked = append(c.Tracked[:i], c.Tracked[i+1:]...)
			return true
		}
	}
	return false
}
