package server

import (
	"archivus/internal/auth"
	"archivus/pkg/response"
	"encoding/json"
	"net/http"
)

func Login(w http.ResponseWriter, r *http.Request) {
	// Parse the login request
	var loginReq auth.LoginUserRequest
	err := json.NewDecoder(r.Body).Decode(&loginReq)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the login credentials
	token, userId, err := auth.LoginUser(loginReq)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate a session token for the user
	response.JSONResponse(w, map[string]interface{}{
		"token": token, "user_id": userId,
	})

}
