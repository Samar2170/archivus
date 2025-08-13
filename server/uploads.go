package server

import (
	"archivus/internal/service"
	"archivus/pkg/logging"
	"archivus/pkg/response"
	"net/http"
)

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
