package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/auth"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"net/http"
)

type ApiKeyHandler struct {
	service *auth.AuthService
}

func NewApiKeyHandler(service *auth.AuthService) *ApiKeyHandler {
	return &ApiKeyHandler{service: service}
}

type createApiKeyRequest struct {
	Name   string             `json:"name"`
	Access models.AccessLevel `json:"access"`
}

func (h *ApiKeyHandler) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	var req createApiKeyRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	created, err := h.service.CreateApiKey(userID, req.Name, req.Access)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	// The plaintext key is only ever shown in this response.
	response.JSONResponse(w, created)
}

func (h *ApiKeyHandler) GetApiKeys(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}

	keys, err := h.service.GetApiKeys(userID)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"api_keys": keys})
}

func (h *ApiKeyHandler) RevokeApiKey(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "user ID not found in context")
		return
	}
	type revokeApiKeyRequest struct {
		KeyID string `json:"key_id"`
	}
	var req revokeApiKeyRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	if err := h.service.RevokeApiKey(userID, req.KeyID); err != nil {
		response.NotFoundResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]string{"message": "api key revoked"})
}
