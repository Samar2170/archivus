package server

import (
	"archivus/internal/helpers"
	"archivus/internal/service"
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
		return
	}
	err = helpers.CreateFolder(username, req.Folder)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.SuccessResponse(w, "Folder created successfully")
}

// func MoveFolderHandler(w http.ResponseWriter, r *http.Request) {
// 	userId := r.Header.Get("userId")
// 	type MoveFolderRequest struct {
// 		Folder string `json:"folder"`
// 		Dest   string `json:"dest"`
// 	}
// 	var req MoveFolderRequest
// 	err := reqhelpers.DecodeRequest(r, &req)
// 	if err != nil {
// 		logging.Errorlogger.Error().Msg(err.Error())
// 		response.BadRequestResponse(w, "Invalid request body")
// 		return
// 	}
// 	if req.Folder == "" || req.Dest == "" {
// 		response.BadRequestResponse(w, "Folder and destination are required")
// 		return
// 	}
// 	// err = service.MoveFolder(userId, req.Folder, req.Dest)
// 	// if err != nil {
// 	// 	logging.Errorlogger.Error().Msg(err.Error())
// 	// 	response.InternalServerErrorResponse(w, err.Error())
// 	// 	return
// 	// }
// 	response.SuccessResponse(w, "Folder moved successfully")
// }

func DeleteFolderHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	type DeleteFolderRequest struct {
		Folder string `json:"folder"`
	}
	var req DeleteFolderRequest
	err := reqhelpers.DecodeRequest(r, &req)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, "Invalid request body")
		return
	}
	if req.Folder == "" {
		response.BadRequestResponse(w, "Folder path is required")
		return
	}
	err = service.DeleteFolder(userId, req.Folder)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.SuccessResponse(w, "Folder deleted successfully")
}
