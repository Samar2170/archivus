package server

import (
	"archivus/internal/helpers"
	"archivus/pkg/logging"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"net/http"
)

func CreateFolderHandler(w http.ResponseWriter, r *http.Request) {
	username := r.Header.Get("username")
	type CreateFolderRequest struct {
		Folder string
	}
	var req CreateFolderRequest
	err := reqhelpers.DecodeRequest(r, &req)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, "Invalid request body")
		return
	}
	if req.Folder == "" {
		response.BadRequestResponse(w, "Folder path is required")
	}
	err = helpers.CreateFolder(username, req.Folder)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.SuccessResponse(w, "Folder created successfully")
}
