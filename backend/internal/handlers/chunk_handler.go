package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/chunkupload"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
)

// Chunked upload flow (resumable, survives mid-upload network drops):
//
//  1. POST /storage/file/upload/chunk/init      -> returns a server-generated uploadId
//  2. POST /storage/file/upload/chunk/part      -> upload each chunk (idempotent; safe to re-send)
//  3. GET  /storage/file/upload/chunk/status    -> which chunk indexes have landed (resume point)
//  4. POST /storage/file/upload/chunk/complete  -> assemble + persist the file
//  5. POST /storage/file/upload/chunk/abort     -> discard an unfinished upload
//
// After a disconnect the client calls status, learns which indexes are missing,
// and re-sends only those before completing. Each call is scoped to the calling
// user: a session can only be touched by the user who created it.

// loadOwnedSession fetches the session for uploadId and verifies it belongs to
// userID. On any failure it writes the appropriate HTTP response and returns
// ok=false, so callers can simply `if !ok { return }`.
func (h *StorageHandler) loadOwnedSession(w http.ResponseWriter, uploadID, userID string) (chunkupload.Session, bool) {
	sess, err := h.chunks.Load(uploadID)
	if err != nil {
		if errors.Is(err, chunkupload.ErrSessionNotFound) {
			response.NotFoundResponse(w, "upload session not found")
		} else {
			response.BadRequestResponse(w, err.Error())
		}
		return chunkupload.Session{}, false
	}
	if sess.UserID != userID {
		// Don't disclose that the session exists to a non-owner.
		response.NotFoundResponse(w, "upload session not found")
		return chunkupload.Session{}, false
	}
	return sess, true
}

// InitChunkUploadHandler registers a new chunked upload session and returns its
// uploadId along with any chunks already received (always empty for a fresh
// session, but shaped like the status response for client convenience).
func (h *StorageHandler) InitChunkUploadHandler(w http.ResponseWriter, r *http.Request) {
	type initRequest struct {
		Filename    string `json:"filename"`
		DriveId     string `json:"driveId"`
		FolderPath  string `json:"folderPath"`
		ContentType string `json:"contentType"`
		Size        int64  `json:"size"`
		TotalChunks int    `json:"totalChunks"`
	}
	var req initRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	if req.Filename == "" || req.DriveId == "" {
		response.BadRequestResponse(w, "filename and driveId are required")
		return
	}
	if req.TotalChunks <= 0 {
		response.BadRequestResponse(w, "totalChunks must be a positive integer")
		return
	}
	if req.Size > archivus_constants.MaxUploadSize {
		response.BadRequestResponse(w, "file exceeds maximum upload size")
		return
	}

	sess, err := h.chunks.Init(chunkupload.Session{
		UserID:      userID,
		DriveID:     req.DriveId,
		FolderPath:  req.FolderPath,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		Size:        req.Size,
		TotalChunks: req.TotalChunks,
	})
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{
		"uploadId":    sess.UploadID,
		"totalChunks": sess.TotalChunks,
		"received":    []int{},
	})
}

// UploadChunkHandler receives a single chunk of an in-progress upload. The chunk
// bytes are sent as the multipart file field "chunk"; "uploadId" and
// "chunkIndex" are form values. Re-sending a chunk is safe and idempotent.
func (h *StorageHandler) UploadChunkHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, archivus_constants.MaxChunkSize)
	if err := r.ParseMultipartForm(archivus_constants.MaxMultipartMemory); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	uploadID := r.FormValue("uploadId")
	chunkIndex, err := strconv.Atoi(r.FormValue("chunkIndex"))
	if err != nil {
		response.BadRequestResponse(w, "chunkIndex must be an integer")
		return
	}
	sess, ok := h.loadOwnedSession(w, uploadID, userID)
	if !ok {
		return
	}

	file, _, err := r.FormFile("chunk")
	if err != nil {
		response.BadRequestResponse(w, "missing chunk file field")
		return
	}
	defer file.Close()

	if err := h.chunks.SaveChunk(sess, chunkIndex, file); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	received, err := h.chunks.ReceivedChunks(sess)
	if err != nil {
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{
		"uploadId":    sess.UploadID,
		"received":    received,
		"totalChunks": sess.TotalChunks,
	})
}

// ChunkUploadStatusHandler reports which chunk indexes have been received so a
// client can resume an interrupted upload by re-sending only the missing ones.
func (h *StorageHandler) ChunkUploadStatusHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	uploadID := r.URL.Query().Get("uploadId")
	sess, ok := h.loadOwnedSession(w, uploadID, userID)
	if !ok {
		return
	}
	received, err := h.chunks.ReceivedChunks(sess)
	if err != nil {
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{
		"uploadId":    sess.UploadID,
		"filename":    sess.Filename,
		"received":    received,
		"totalChunks": sess.TotalChunks,
		"complete":    len(received) == sess.TotalChunks,
	})
}

// CompleteChunkUploadHandler assembles all received chunks into the final file
// and persists it through the normal storage path (which enforces drive write
// access and handles overwrite/versioning per backend). On success the staging
// directory is removed.
func (h *StorageHandler) CompleteChunkUploadHandler(w http.ResponseWriter, r *http.Request) {
	type completeRequest struct {
		UploadId string `json:"uploadId"`
	}
	var req completeRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	sess, ok := h.loadOwnedSession(w, req.UploadId, userID)
	if !ok {
		return
	}

	assembled, err := h.chunks.Assemble(sess)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	defer os.Remove(assembled.Name())
	defer assembled.Close()

	info, err := assembled.Stat()
	if err != nil {
		response.InternalServerErrorResponse(w, err.Error())
		return
	}

	// UploadFileV2 works against multipart.File / *multipart.FileHeader; an
	// *os.File satisfies multipart.File, and we hand-build the header from the
	// session metadata so the assembled file flows through the same persistence
	// path as a direct multipart upload.
	fileHeader := &multipart.FileHeader{
		Filename: sess.Filename,
		Size:     info.Size(),
		Header:   textproto.MIMEHeader{},
	}
	if sess.ContentType != "" {
		fileHeader.Header.Set("Content-Type", sess.ContentType)
	}

	if err := h.service.UploadFileV2(sess.FolderPath, sess.DriveID, userID, assembled, fileHeader); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	if err := h.chunks.Discard(sess.UploadID); err != nil {
		// The file is already persisted; a stale staging dir is non-fatal.
		fmt.Printf("warning: failed to clean up chunk staging for %q: %v\n", sess.UploadID, err)
	}
	response.JSONResponse(w, map[string]string{
		"message":  "file uploaded successfully",
		"filename": sess.Filename,
	})
}

// AbortChunkUploadHandler discards an in-progress upload session and its staged
// chunks.
func (h *StorageHandler) AbortChunkUploadHandler(w http.ResponseWriter, r *http.Request) {
	type abortRequest struct {
		UploadId string `json:"uploadId"`
	}
	var req abortRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	sess, ok := h.loadOwnedSession(w, req.UploadId, userID)
	if !ok {
		return
	}
	if err := h.chunks.Discard(sess.UploadID); err != nil {
		response.InternalServerErrorResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"message": "upload aborted"})
}
