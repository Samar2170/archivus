package server

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	"archivus/pkg/response"
	"context"
	"net/http"
	"strings"
)

var allowedOrigins = map[string]bool{
	"http://localhost:3000": true,
	"http://localhost:5173": true,
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func HomeMiddleware(as *auth.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

func AuthMiddleware(as *auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				response.UnauthorizedResponse(w, "missing or invalid authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			userID, _, err := as.DecodeToken(token)
			if err != nil {
				response.UnauthorizedResponse(w, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), archivus_constants.ContextKey(archivus_constants.UserIdKey), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
