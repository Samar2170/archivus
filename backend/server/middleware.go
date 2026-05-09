package server

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	"archivus/pkg/response"
	"context"
	"net/http"
	"strings"
)

// var exemptedPaths = map[string]bool{
// 	"/health":        true,
// 	"/auth/login":    true,
// 	"/auth/register": true,
// }

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// if exemptedPaths[r.URL.Path] {
		// 	next.ServeHTTP(w, r)
		// 	return
		// }
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.UnauthorizedResponse(w, "missing or invalid authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, _, err := auth.DecodeToken(token)
		if err != nil {
			response.UnauthorizedResponse(w, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), archivus_constants.ContextKey(archivus_constants.UserIdKey), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
