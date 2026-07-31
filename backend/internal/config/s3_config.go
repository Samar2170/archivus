package config

import (
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

// s3ConfigPaths returns the candidate locations for the s3 config file, most
// specific first: ARCHIVUS_S3_CONFIG if set, then the settings directory
// (co-located with config.yaml), then the project/home directory.
func s3ConfigPaths(home, root string) []string {
	var paths []string
	if p := os.Getenv("ARCHIVUS_S3_CONFIG"); p != "" {
		paths = append(paths, p)
	}
	return append(paths,
		filepath.Join(root, s3ConfigFileName),
		filepath.Join(home, s3ConfigFileName),
	)
}

const s3ConfigFileName = "s3_config.yaml"
