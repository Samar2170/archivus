package server

import (
	"archivus/internal/service"
	"archivus/pkg/logging"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"net/http"
)

var allowOrderBy = map[string]string{
	"name":    "name",
	"size":    "size",
	"created": "created_at",
}

func orderingHelper(orderBy string) (string, string) {
	ordering := "ASC"
	if len(orderBy) != 0 && orderBy[0] == '-' {
		orderBy = orderBy[1:] // Remove the '-' prefix for descending order
		ordering = "DESC"
	}
	col, ok := allowOrderBy[orderBy]
	if !ok {
		logging.Errorlogger.Error().Msgf("Invalid order_by parameter: %s", orderBy)
		col = "created_at"
	}
	return col, ordering
}

func GetFilesHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	orderBy := r.URL.Query().Get("order_by")
	pageNo := r.URL.Query().Get("page")
	search := r.URL.Query().Get("search")
	orderBy, ordering := orderingHelper(orderBy)

	files, err := service.FindFiles(userId, search, orderBy, ordering, pageNo)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	data := map[string]interface{}{"files": files}
	response.JSONResponse(w, data)
}

func GetFilesByFolder(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	folder := r.URL.Query().Get("folder")
	files, folderSize, err := service.GetFiles(userId, folder)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	// for _, file := range files {
	// file.NavigationPath = fmt.Sprintf("http://%s/app/?folder=%s", config.GetFrontendAddr(), file.Path)
	// }
	response.JSONResponse(w, map[string]interface{}{
		"files": files,
		"size":  folderSize,
	})

}

type MoveFileRequest struct {
	FilePath string `json:"filePath"`
	Dst      string `json:"dst"`
}

func MoveFileHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	var req MoveFileRequest
	err := reqhelpers.DecodeRequest(r, &req)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, "Invalid request body")
		return
	}
	err = service.MoveFile(userId, req.FilePath, req.Dst, false)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"message": "File moved successfully"})
}

type DeleteFileRequest struct {
	FilePath string `json:"filePath"`
}

func DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Header.Get("userId")
	var req DeleteFileRequest
	err := reqhelpers.DecodeRequest(r, &req)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.BadRequestResponse(w, "Invalid request body")
		return
	}

	err = service.DeleteFile(userId, req.FilePath)
	if err != nil {
		logging.Errorlogger.Error().Msg(err.Error())
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"message": "File deleted successfully"})
}
