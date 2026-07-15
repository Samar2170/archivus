package server

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	"archivus/pkg/response"
	"context"
	"encoding/json"
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

		// WebDAV clients use OPTIONS to discover capabilities (the DAV header),
		// so let those requests reach the webdav handler instead of answering
		// them here as a CORS preflight.
		if r.Method == http.MethodOptions && !strings.HasPrefix(r.URL.Path, "/webdav/") {
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

// BasicAuthMiddleware authenticates WebDAV clients via HTTP Basic Auth and
// stores the resolved user ID in the request context (same key as the REST
// AuthMiddleware). On failure it sends a WWW-Authenticate challenge so native
// clients (Finder, Explorer, rclone) prompt for credentials.
// basicAuthChallenge sends a 401 with a WWW-Authenticate header. Native WebDAV
// clients (Finder, Explorer, Dolphin/KIO, rclone) first probe without
// credentials and only prompt for / send them when they receive a 401. A 403
// tells them access is forbidden outright, so they never authenticate and the
// mount appears to "not exist".
func basicAuthChallenge(w http.ResponseWriter, err string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="archivus"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Unauthorized",
		"error":   err,
	})
}

func BasicAuthMiddleware(as *auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok {
				basicAuthChallenge(w, "missing basic auth credentials")
				return
			}
			userID, err := as.Authenticate(username, password)
			if err != nil {
				basicAuthChallenge(w, "invalid credentials")
				return
			}
			ctx := context.WithValue(r.Context(), archivus_constants.ContextKey(archivus_constants.UserIdKey), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
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
