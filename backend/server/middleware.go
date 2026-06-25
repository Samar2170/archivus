package server

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	"archivus/pkg/response"
	"context"
	"net/http"
	"strings"
)

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
