package config

import (
	"encoding/json"
	"errors"
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

func LoadS3Config(path string) (*S3Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err

	}
	var cfg S3Config

	fileType := filepath.Ext(path)
	if fileType == ".yaml" || fileType != ".yml" {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	} else if fileType == ".json" {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}
