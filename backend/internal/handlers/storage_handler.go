package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/storagemanager"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"net/http"
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
