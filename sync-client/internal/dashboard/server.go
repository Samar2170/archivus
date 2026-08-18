// Package dashboard serves a small read-only web dashboard for the sync

// client's state. It is intentionally standard-library only so it doesn't add
// any dependency to the module. It exposes JSON endpoints that avoid leaking
// the bearer token from config.json.
package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"

	"archivus-sync/internal/config"
)

//go:embed index.html
var assets embed.FS

// Server serves the dashboard over HTTP.
type Server struct {
	cfg *config.Config
}

// New creates a dashboard server for the given config.
func New(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/config":
		s.handleConfig(w, r)
	case "/api/state":
		s.handleState(w, r)
	default:
		serveIndex(w, r)
	}
}

// handleConfig returns the login/username/tracked folders without the token.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"serverUrl": s.cfg.ServerURL,
		"username":  s.cfg.Username,
		"tracked":   s.cfg.Tracked,
	})
}

// handleState reads state.json from the sync home directory. It mirrors the
// persisted schema in internal/syncer/state.go: {"files": {...}, "pending": {...}}.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	statePath, err := config.StatePathForServer(s.cfg.ServerURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		writeJSON(w, map[string]any{
			"files":   map[string]any{},
			"pending": map[string]any{},
		})
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if state == nil {
		state = map[string]any{}
	}
	if state["files"] == nil {
		state["files"] = map[string]any{}
	}
	if state["pending"] == nil {
		state["pending"] = map[string]any{}
	}
	writeJSON(w, state)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	html, _ := fs.ReadFile(assets, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
