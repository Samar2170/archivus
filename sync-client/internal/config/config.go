// Package config manages the archivus-sync client's on-disk configuration:
// the server URL, auth token, and the list of tracked folders. Everything lives
// under a per-user directory (~/.archivus-sync by default, overridable with
// ARCHIVUS_SYNC_HOME) so the client is self-contained on a desktop.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DirName is the client's settings directory under the user's home.
	DirName = ".archivus-sync"
	// FileName is the config file within DirName.
	FileName = "config.json"
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
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Load reads the config file. A missing file yields an empty Config so first-run
// commands can populate and save it.
func Load() (*Config, error) {
	path, err := Path()
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
	return &c, nil
}

// Save writes the config file atomically with owner-only permissions (it holds a
// bearer token).
func (c *Config) Save() error {
	path, err := Path()
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
	return nil
}

// LoggedIn reports whether a server URL and token are configured.
func (c *Config) LoggedIn() bool {
	return c.ServerURL != "" && c.Token != ""
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
