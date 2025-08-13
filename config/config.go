package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type HostingConfiguration struct {
	BaseUrl string `yaml:"base_url"`
	Port    string `yaml:"port"`
	Scheme  string `yaml:"scheme"`
}

type Configuration struct {
	Mode            string               `yaml:"mode"`
	LogsDir         string               `yaml:"logs_dir"`
	StorageDbFile   string               `yaml:"storage_db_file"`
	SecretKey       string               `yaml:"secret_key"`
	BotToken        string               `yaml:"bot_token"`         // Added BotToken to the configuration
	AdminAccountPin string               `yaml:"admin_account_pin"` // Added AdminAccountPin to the configuration
	UploadsDir      string               `yaml:"uploads_dir"`
	BackendConfig   HostingConfiguration `yaml:"backend_config"`
	FrontEndConfig  HostingConfiguration `yaml:"frontend_config"`
}

var Config *Configuration

func LoadConfig(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, &Config); err != nil {
		return err
	}
	if Config == nil {
		Config = &Configuration{}
	}
	if Config.LogsDir == "" {
		Config.LogsDir = "logs"
	}
	if Config.StorageDbFile == "" {
		Config.StorageDbFile = "storage.db"
	}
	if Config.UploadsDir != "" && !filepath.IsAbs(Config.UploadsDir) {
		Config.UploadsDir = filepath.Join(BaseDir, Config.UploadsDir)
	}
	if Config.UploadsDir == "" {
		Config.UploadsDir = filepath.Join(BaseDir, "uploads")
	}
	return nil
}

var BaseDir string

const (
	HeaderName         = "AccessKey"
	CompressionQuality = 85
)

func init() {
	var err error
	// currentFile, err := os.Executable()
	// if err != nil {
	// 	panic(err)
	// }

	// BaseDir = filepath.Dir(currentFile)
	BaseDir = "/Users/samararora/Desktop/PROJECTS/archivus/"
	err = LoadConfig(BaseDir + "config.yaml")
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}
	CheckConfig()

}
func CheckConfig() {
	if Config == nil {
		panic("Configuration is nil after loading")
	}
	if Config.Mode == "" {
		panic("Mode is not set in the configuration")
	}
	if Config.SecretKey == "" {
		panic("SecretKey is not set in the configuration")
	}
	if Config.BackendConfig.BaseUrl == "" ||
		Config.BackendConfig.Port == "" ||
		Config.FrontEndConfig.BaseUrl == "" ||
		Config.FrontEndConfig.Port == "" {
		panic("Backend or Frontend configuration is incomplete")
	}

}
