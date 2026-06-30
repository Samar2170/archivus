package server

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	"archivus/internal/webdavfs"
	"archivus/pkg/response"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/net/webdav"
)

// WebDAVHandler exposes each drive as a WebDAV mount at /webdav/{slug}/...,
// backed by the same StorageManager and metadata store as the REST API.
type WebDAVHandler struct {
	authService *auth.AuthService
	locks       webdav.LockSystem
}

func NewWebDAVHandler(as *auth.AuthService) *WebDAVHandler {
	return &WebDAVHandler{
		authService: as,
		// In-memory locks are per-process; fine for a single-instance server.
		locks: webdav.NewMemLS(),
	}
}

func (h *WebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}

	drive, err := h.authService.Store.GetDriveBySlug(slug)
	if err != nil {
		http.Error(w, "drive not found", http.StatusNotFound)
		return
	}

	dav := &webdav.Handler{
		Prefix:     "/webdav/" + slug,
		FileSystem: webdavfs.New(h.authService.StorageManager, drive.ID.String(), userID),
		LockSystem: h.locks,
	}
	dav.ServeHTTP(w, r)
}
