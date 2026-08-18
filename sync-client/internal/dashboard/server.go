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
	"sort"
	"strconv"

	"archivus-sync/internal/config"
)

//go:embed index.html
var assets embed.FS

const (
	defaultPageSize = 50
	maxPageSize     = 500
)

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
	case "/api/files":
		s.handleFiles(w, r)
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

// handleState returns the summary counts plus the pending uploads, without the
// full files map (that is served page by page via /api/files).
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	files, pending, err := s.readState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"totalFiles": len(files),
		"pending":    pending,
	})
}

// handleFiles serves one sorted page of the recorded files. It reads state.json
// from the sync home directory; the schema lives in internal/syncer/state.go.
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	files, _, err := s.readState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	offset := parseIntQuery(r, "offset", 0)
	limit := parseIntQuery(r, "limit", defaultPageSize)
	if limit < 1 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	start := offset
	if start < 0 {
		start = 0
	}
	if start > len(keys) {
		start = len(keys)
	}
	end := start + limit
	if end > len(keys) {
		end = len(keys)
	}

	out := make([]map[string]any, 0, end-start)
	for _, k := range keys[start:end] {
		item := map[string]any{"path": k}
		if m, ok := files[k].(map[string]any); ok {
			item["size"] = m["size"]
			item["modTime"] = m["modTime"]
			item["checksum"] = m["checksum"]
		}
		out = append(out, item)
	}

	writeJSON(w, map[string]any{
		"total":  len(keys),
		"offset": start,
		"files":  out,
	})
}

// readState loads state.json and returns its files and pending maps (defaulting
// to empty maps when the file or either section is absent).
func (s *Server) readState() (files, pending map[string]any, err error) {
	statePath, err := config.StatePathForServer(s.cfg.ServerURL)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return map[string]any{}, map[string]any{}, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, nil, err
	}
	if state == nil {
		state = map[string]any{}
	}

	files, _ = state["files"].(map[string]any)
	if files == nil {
		files = map[string]any{}
	}
	pending, _ = state["pending"].(map[string]any)
	if pending == nil {
		pending = map[string]any{}
	}
	return files, pending, nil
}

func parseIntQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
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
