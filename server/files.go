package server

import (
	"archivus/internal/helpers"
	"archivus/internal/service"
	"archivus/pkg/logging"
	"archivus/pkg/response"
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

func GetFilesHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	files, err := service.GetFiles(userId)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, files)
}

func UploadFilesHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20) // 32 MB max memory, adjust as needed
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, err.Error())
		return
	}

	username := r.Header.Get("username")
	folderPath := r.FormValue("folder")

	// Get multiple files with field name "file"
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		response.BadRequestResponse(w, "No files provided")
		return
	}

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			logging.Errorlogger.Error().Msg(err.Error())
			response.BadRequestResponse(w, "Error opening file: "+fileHeader.Filename)
			return
		}

		err = service.SaveFile(file, fileHeader, username, folderPath, "")
		file.Close()

		if err != nil {
			logging.Errorlogger.Error().Msg(err.Error())
			response.InternalServerErrorResponse(w, "Error saving file: "+fileHeader.Filename)
			return
		}
	}
	response.SuccessResponse(w, "File uploaded successfully")
}

func CreateFolderHandler(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("username")
	folderPath := r.FormValue("folder")
	if folderPath == "" {
		response.BadRequestResponse(w, "Folder path is required")
	}
	err := helpers.CreateFolder(username, folderPath)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.SuccessResponse(w, "Folder created successfully")
}

func GetSignedUrlHandler(w http.ResponseWriter, r *http.Request) {
	filepath := mux.Vars(r)["filepath"]
	userId := r.Header.Get("userId")
	signedUrl, err := service.GetSignedUrl(filepath, userId)
	// ownership check please
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.SuccessResponse(w, signedUrl)
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	compressed := false
	filepath := mux.Vars(r)["filepath"]
	signature := r.URL.Query().Get("signature")
	expiresAtStr := r.URL.Query().Get("expires_at")
	compressedStr := r.URL.Query().Get("compressed")
	if compressedStr == "true" {
		compressed = true
	}
	expiresAt, err := strconv.Atoi(expiresAtStr)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, err.Error())
		return
	}
	if time.Now().After(time.Unix(int64(expiresAt), 0)) {
		response.UnauthorizedResponse(w, "Signature expired")
		return
	}
	f, err := service.DownloadFile(filepath, signature, expiresAtStr, compressed)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	http.ServeContent(w, r, filepath, time.Now(), bytes.NewReader(f))
}
