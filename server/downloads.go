package server

import (
	"archivus/internal/service"
	"archivus/pkg/logging"
	"archivus/pkg/response"
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

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
	response.JSONResponse(w, map[string]interface{}{
		"signed_url": signedUrl})
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
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, filepath, time.Now(), bytes.NewReader(f))
}
