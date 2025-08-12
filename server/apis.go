package server

import (
	"archivus/config"
	"archivus/internal/middleware"
	"archivus/pkg/logging"
	"net/http"

	"github.com/gorilla/mux"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func GetServer() *http.Server {
	logger := logging.AuditLogger
	mux := mux.NewRouter()
	healthCheckHandler := http.HandlerFunc(HealthCheck)
	getFilesHandler := http.HandlerFunc(GetFilesHandler)
	getSignedUrlHandler := http.HandlerFunc(GetSignedUrlHandler)
	downloadHandler := http.HandlerFunc(DownloadFileHandler)
	uploadFilesHandler := http.HandlerFunc(UploadFilesHandler)

	mux.HandleFunc("/health-check/", healthCheckHandler)
	mux.HandleFunc("/files/get/", getFilesHandler)
	mux.HandleFunc("/files/get-signed-url/{filepath:.*}", getSignedUrlHandler)
	mux.HandleFunc("/files/download/{filepath:.*}", downloadHandler)
	mux.HandleFunc("/folder/add/", CreateFolderHandler).Methods("POST")

	mux.HandleFunc("/files/upload/", uploadFilesHandler).Methods("POST")

	logMiddleware := logging.NewLogMiddleware(&logger)
	mux.Use(logMiddleware.Func())

	wrappedMux := middleware.APIKeyMiddleware(mux)
	wrappedMux = CorsConfig.Handler(wrappedMux)
	server := http.Server{
		Handler: wrappedMux,
		Addr:    config.GetBackendAddr(),
	}
	return &server
}
