package config

import (
	archivus_constants "archivus/internal/constants"
	"fmt"
	"os"
)

type Configuration struct {
	DefaultWriteAccess bool   `yaml:"default_write_access"` // default write access for users, if false, users will have read-only access by default
	AllowUserDrive     bool   `yaml:"allow_user_drive"`     // allow users to have their own drive with write access, if false, users will not have their own drive and will have read-only access to the shared drive
	LogsDir            string `yaml:"logs_dir"`
	SecretKey          string `yaml:"secret_key"`

	BaseDir string `yaml:"base_dir"`
	// AllowedOrigins  []string `yaml:"allowed_origins"`
	ServerSalt      string `yaml:"server_salt"`
	BackendProxyUrl string `yaml:"backend_proxy_url"`
}

var Config *Configuration

func (c *Configuration) Print() {
	fmt.Println("Configuration:")
	fmt.Printf("  DefaultWriteAccess: %v\n", c.DefaultWriteAccess)
	fmt.Printf("  LogsDir: %s\n", c.LogsDir)
	fmt.Printf("  SecretKey: %s\n", c.SecretKey)
	fmt.Printf("  BaseDir: %s\n", c.BaseDir)
	fmt.Printf("  ServerSalt: %s\n", c.ServerSalt)
	// fmt.Printf("  AllowedOrigins: %v\n", c.AllowedOrigins)

}

var ProjectBaseDir string

func setupConfig(mode string) {
	if mode == "dev" {
		// currentFile, err := os.Executable()
		// if err != nil {
		// 	panic(err)
		// }
		// ProjectBaseDir = filepath.Dir(currentFile)
		ProjectBaseDir = "./"
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		ProjectBaseDir = homeDir + "/" + archivus_constants.SettingsDir
		err = os.MkdirAll(ProjectBaseDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	sk, err := generateRandomAlphaNumericString(32)
	if err != nil {
		panic("Failed to generate secret key: " + err.Error())
	}
	ss, err := generateRandomAlphaNumericString(16)
	if err != nil {
		panic("Failed to generate server salt: " + err.Error())
	}
	configuration := &Configuration{
		DefaultWriteAccess: true,
		AllowUserDrive:     true,
		LogsDir:            "logs",
		SecretKey:          sk,
		BaseDir:            "archivus",
		ServerSalt:         ss,
	}
	saveConfig(configuration)
}

func saveConfig(config *Configuration) error {
	configFilePath := ProjectBaseDir + "/" + archivus_constants.ConfigFileName
	err := os.WriteFile(configFilePath, []byte(fmt.Sprintf("default_write_access: %v\nlogs_dir: %s\nsecret_key: %s\nbase_dir: %s\nserver_salt: %s\nallowed_origins:\n%s\nbackend_proxy_url: %s\n",
		config.DefaultWriteAccess,
		config.LogsDir,
		config.SecretKey,
		config.BaseDir,
		config.ServerSalt,
		config.BackendProxyUrl,
	)), 0644)
	return err
}

func CheckConfig() error {
	configFilePath := ProjectBaseDir + "/" + archivus_constants.ConfigFileName
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		fmt.Println("Config file not found, creating default config...")
		setupConfig("prod")
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking config file: %v", err)
	}
	return nil
}
