package config

import (
	archivus_constants "archivus/internal/constants"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var ErrUnsupportedFileType = errors.New("unsupported file type, for s3 config only yaml and json are supported")

type S3Config struct {
	AccountID  string `yaml:"account_id" json:"account_id"`
	S3API      string `yaml:"s3_api" json:"s3_api"`
	AccessKey  string `yaml:"access_key" json:"access_key"`
	SecretKey  string `yaml:"secret_key" json:"secret_key"`
	BucketName string `yaml:"bucket_name" json:"bucket_name"`
}

// LoadS3Config reads the s3 config from the first path in paths that exists,
// decoding it as yaml or json based on the file extension.
func LoadS3Config(paths []string) (*S3Config, error) {
	var path string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no s3 config file found in: %v", paths)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg S3Config
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedFileType
	}

	return &cfg, nil
}

// DefaultS3Paths returns the path to the s3 config file. It looks in the
// .archivus settings directory first, then falls back to the project/home
// directory, returning the first location that exists. If neither exists it
// returns the settings-directory path (co-located with config.yaml).
func DefaultS3Paths() ([]string, error) {
	var err error
	var projectDir string
	if DEBUG {
		projectDir, err = os.Getwd()
	} else {
		projectDir, err = os.UserHomeDir()
	}
	if err != nil {
		return []string{""}, err
	}

	candidates := []string{
		filepath.Join(projectDir, archivus_constants.SettingsDir, "s3_config.yaml"),
		filepath.Join(projectDir, "s3_config.yaml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return candidates, nil
		}
	}
	return candidates, nil
}
