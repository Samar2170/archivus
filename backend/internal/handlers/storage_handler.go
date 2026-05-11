package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/storagemanager"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type StorageHandler struct {
	service *storagemanager.StorageManager
}

func NewStorageHandler(service *storagemanager.StorageManager) *StorageHandler {
	return &StorageHandler{service: service}
}

func (h *StorageHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	type createFolderRequest struct {
		Path    string `json:"path"`
		DriveId string `json:"driveId"`
	}
	var req createFolderRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}

	if err := h.service.CreateDir(req.Path, req.DriveId, userID); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
}

func (h *StorageHandler) DeleteFolder(w http.ResponseWriter, r *http.Request) {
	type deleteFolderRequest struct {
		Path    string `json:"path"`
		DriveId string `json:"driveId"`
	}
	var req deleteFolderRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	if err := h.service.DeleteDir(req.Path, req.DriveId, userID); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"message": "folder deleted"})
}

func (h *StorageHandler) UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, archivus_constants.MaxUploadSize)
	err := r.ParseMultipartForm(archivus_constants.MaxUploadSize)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	folderPath := r.FormValue("folderPath")
	driveID := r.FormValue("driveId")

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		response.BadRequestResponse(w, "no files uploaded")
		return
	}

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			response.BadRequestResponse(w, err.Error())
			return
		}
		defer file.Close()

		if err := h.service.UploadFile(folderPath, driveID, userID, file, fileHeader); err != nil {
			response.BadRequestResponse(w, err.Error())
			return
		}
	}
	response.JSONResponse(w, map[string]string{"message": "files uploaded successfully"})
}

func (h *StorageHandler) MoveFileHandler(w http.ResponseWriter, r *http.Request) {
	type moveFileRequest struct {
		SrcPath string `json:"srcPath"`
		DstPath string `json:"dstPath"`
		DriveId string `json:"driveId"`
	}
	var req moveFileRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	if err := h.service.MoveFile(req.SrcPath, req.DstPath, req.DriveId, userID); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"message": "file moved successfully"})
}

func (h *StorageHandler) DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	fileIdStr := r.URL.Query().Get("fileId")
	driveId := r.URL.Query().Get("driveId")

	fileId, err := strconv.ParseInt(fileIdStr, 10, 64)
	if err != nil {
		response.BadRequestResponse(w, "invalid file ID")
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}

	file, md, err := h.service.DownloadFile(fileId, driveId, userID)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", md.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, md.Name, time.Time{}, file)
}

func (h *StorageHandler) GetFilesHandler(w http.ResponseWriter, r *http.Request) {
	type getFilesRequest struct {
		Path    string `json:"path"`
		DriveId string `json:"driveId"`
	}
	var req getFilesRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	files, err := h.service.GetFiles(req.Path, req.DriveId, userID)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{
		"files": files,
	})
}
