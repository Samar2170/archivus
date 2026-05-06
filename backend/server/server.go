package server

import (
	"archivus/pkg/response"
	"net/http"

	"github.com/gorilla/mux"
)

const (
	UserIdKey = "userId"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.SuccessResponse(w, "OK")
}

func GetServer(testEnv bool) *http.Server {
	mux := mux.NewRouter()
	mux.HandleFunc("/health", HealthCheck)

	server := &http.Server{
		Handler: mux,
	}
	return server
}
