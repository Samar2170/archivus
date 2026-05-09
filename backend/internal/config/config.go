package config

import (
	archivus_constants "archivus/internal/constants"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// should be false in production
var DEBUG = true

type Configuration struct {
	DefaultWriteAccess bool   `yaml:"default_write_access"`
	AllowUserDrive     bool   `yaml:"allow_user_drive"`
	LogsDir            string `yaml:"logs_dir"`
	SecretKey          string `yaml:"secret_key"`
	ArchivusHome       string `yaml:"base_dir"`
	ServerSalt         string `yaml:"server_salt"`
	BackendProxyUrl    string `yaml:"backend_proxy_url"`
}

var (
	Config         *Configuration
	ProjectBaseDir string
)

// Init sets ProjectBaseDir, writes a default config if none exists, then loads
// it into Config. Must be called before any other package that reads Config.
func Init() error {
	var homeDir string
	var err error

	if DEBUG {
		homeDir, err = os.Getwd()
	} else {
		homeDir, err = os.UserHomeDir()
	}

	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	ProjectBaseDir = filepath.Join(homeDir, archivus_constants.SettingsDir)
	if err := os.MkdirAll(ProjectBaseDir, os.ModePerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	configPath := filepath.Join(ProjectBaseDir, archivus_constants.ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg, err := newDefault(homeDir)
		if err != nil {
			return fmt.Errorf("build default config: %w", err)
		}
		if err := save(cfg, configPath); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
	}

	Config, err = load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return nil
}

func newDefault(homeDir string) (*Configuration, error) {
	sk, err := generateRandomAlphaNumericString(32)
	if err != nil {
		return nil, err
	}
	ss, err := generateRandomAlphaNumericString(16)
	if err != nil {
		return nil, err
	}
	// make archivus home dir if not exists
	archivusHome := filepath.Join(homeDir, "archivus")
	if err := os.MkdirAll(archivusHome, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create archivus home dir: %w", err)
	}
	logsDir := filepath.Join(ProjectBaseDir, "logs")
	if err := os.MkdirAll(logsDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	return &Configuration{
		DefaultWriteAccess: false,
		AllowUserDrive:     true,
		LogsDir:            logsDir,
		SecretKey:          sk,
		ArchivusHome:       archivusHome,
		ServerSalt:         ss,
	}, nil
}

func load(path string) (*Configuration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Configuration
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func save(cfg *Configuration, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
