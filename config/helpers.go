package config

import "fmt"

func GetBackendAddr() string {
	return Config.BackendConfig.BaseUrl + ":" + Config.BackendConfig.Port
}
func GetBackendScheme() string {
	return Config.BackendConfig.Scheme
}
func GetFrontendScheme() string {
	return Config.FrontEndConfig.Scheme
}

func GetFrontendAddr() string {
	return fmt.Sprintf("%s:%s", Config.FrontEndConfig.BaseUrl, Config.FrontEndConfig.Port)
}
