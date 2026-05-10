package server

import (
	"archivus/internal/handlers"
	"archivus/internal/services/auth"
	"archivus/pkg/response"
	"net/http"

	"github.com/gorilla/mux"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	response.SuccessResponse(w, "OK")
}

func GetServer(authService *auth.AuthService) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/health", HealthCheck)

	authHandler := handlers.NewAuthHandler(authService)
	router.HandleFunc("/auth/login", authHandler.Login).Methods(http.MethodPost)
	router.HandleFunc("/auth/register", authHandler.Register).Methods(http.MethodPost)

	protected := router.NewRoute().Subrouter()
	protected.Use(AuthMiddleware)
	protected.HandleFunc("/auth/drive/invite", authHandler.InviteUser).Methods(http.MethodPost)
	protected.HandleFunc("/auth/drive/remove", authHandler.RemoveUserFromDrive).Methods(http.MethodPost)
	protected.HandleFunc("/auth/drive/add", authHandler.AddUserToDrive).Methods(http.MethodPost)

	return &http.Server{Handler: router, Addr: ":8080"}
}
