package server

import (
	"archivus/config"
	"archivus/pkg/logging"
	"fmt"

	"github.com/rs/cors"
)

var CorsConfig *cors.Cors

func init() {
	allowedOrigins := []string{""}
	if config.Config.Mode == "dev" || config.Config.Mode == "net" {
		allowedOrigins = append(allowedOrigins, fmt.Sprintf("%s://%s:%s", config.Config.FrontEndConfig.Scheme, config.Config.FrontEndConfig.BaseUrl, config.Config.FrontEndConfig.Port))
		// add all 192.168.*.* IPs for local development
		for i := 1; i < 255; i++ {
			for j := 1; j < 255; j++ {
				allowedOrigins = append(allowedOrigins, fmt.Sprintf("http://192.168.%d.%d:%s", i, j, config.Config.FrontEndConfig.Port))
			}
		}
		allowedOrigins = append(allowedOrigins, "http://localhost:3000")
	}
	logger := cors.Logger(&logging.AuditLogger)
	CorsConfig = cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "PUT", "DELETE"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		Logger:           logger,
	})

}
