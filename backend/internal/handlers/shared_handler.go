package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/shared"
	"archivus/internal/services/storagemanager"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/store"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"fmt"
	"net/http"
	"time"
)

// SharedHandler serves the /storage/shared/* surface: read-only browsing of
// folders shared with the current user, plus share management for drive
// owners/managers/admins. It deliberately shares no code paths with the
// drive-membership storage handlers.
type SharedHandler struct {
	service *shared.Service
}

func NewSharedHandler(s *store.Store, storage storagemanager.StorageManager) *SharedHandler {
	return &SharedHandler{service: &shared.Service{Store: s, Storage: storage}}
}

func userIDFromContext(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok || userID == "" {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return "", false
	}
	return userID, true
}

// GetRoots lists the folders shared with the current user.
func (h *SharedHandler) GetRoots(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	roots, err := h.service.Roots(userID)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"roots": roots})
}

// ListFiles pages a folder inside a shared subtree.
func (h *SharedHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	type listSharedRequest struct {
		DriveId   string `json:"driveId"`
		RootPath  string `json:"rootPath"`
		Path      string `json:"path"`
		Page      int    `json:"page"`
		PageSize  int    `json:"pageSize"`
		Category  string `json:"category"`
		SortBy    string `json:"sortBy"`
		SortOrder string `json:"sortOrder"`
	}
	var req listSharedRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	query := storage_types.ListFilesQuery{
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}
	switch {
	case req.Category == "others":
		query.Others = true
	default:
		if exts, ok := archivus_constants.FilteringExtensionMap[req.Category]; ok {
			query.Extensions = exts
		}
	}
	page, err := h.service.List(userID, req.DriveId, req.RootPath, req.Path, req.Page, req.PageSize, query)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{
		"files":    page.Entries,
		"total":    page.Total,
		"page":     page.Page,
		"pageSize": page.PageSize,
	})
}

// DownloadFile streams a file from inside a shared subtree.
func (h *SharedHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	fileId := r.URL.Query().Get("fileId")
	driveId := r.URL.Query().Get("driveId")
	rootPath := r.URL.Query().Get("rootPath")
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	file, md, err := h.service.Download(userID, driveId, rootPath, fileId)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", md.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, md.Name, time.Time{}, file)
}

// DownloadURL returns a direct URL for a file inside a shared subtree, or ""
// when the backend has no such capability. mode=inline yields a preview URL.
func (h *SharedHandler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	fileId := r.URL.Query().Get("fileId")
	driveId := r.URL.Query().Get("driveId")
	rootPath := r.URL.Query().Get("rootPath")
	inline := r.URL.Query().Get("mode") == "inline"
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	url, err := h.service.DownloadURL(userID, driveId, rootPath, fileId, inline)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"url": url})
}

// GrantShare shares a folder with an existing user (upsert).
func (h *SharedHandler) GrantShare(w http.ResponseWriter, r *http.Request) {
	type grantShareRequest struct {
		DriveId     string             `json:"driveId"`
		RootPath    string             `json:"rootPath"`
		UserId      string             `json:"userId"`
		Username    string             `json:"username"`
		AccessLevel models.AccessLevel `json:"accessLevel"`
	}
	var req grantShareRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	share, err := h.service.Grant(userID, req.DriveId, req.RootPath, req.UserId, req.Username, req.AccessLevel)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"share": share})
}

// RevokeShare removes a user's share on a folder.
func (h *SharedHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	type revokeShareRequest struct {
		DriveId  string `json:"driveId"`
		RootPath string `json:"rootPath"`
		UserId   string `json:"userId"`
		Username string `json:"username"`
	}
	var req revokeShareRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	if err := h.service.Revoke(userID, req.DriveId, req.RootPath, req.UserId, req.Username); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"message": "share revoked"})
}

// ListSharedUsers lists everyone who has a share on a folder.
func (h *SharedHandler) ListSharedUsers(w http.ResponseWriter, r *http.Request) {
	type listSharedUsersRequest struct {
		DriveId  string `json:"driveId"`
		RootPath string `json:"rootPath"`
	}
	var req listSharedUsersRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := userIDFromContext(w, r)
	if !ok {
		return
	}
	users, err := h.service.Users(userID, req.DriveId, req.RootPath)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"users": users})
}
