package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/services/auth"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"net/http"
)

type AuthHandler struct {
	service *auth.AuthService
}

func NewAuthHandler(service *auth.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PIN      string `json:"pin"`
}

type registerRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	PIN        string `json:"pin"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	token, err := h.service.Login(req.Username, req.Password, req.PIN)
	if err != nil {
		response.UnauthorizedResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]string{"token": token})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	invite, err := h.service.ValidateInviteCode(req.InviteCode)
	if err != nil {
		response.BadRequestResponse(w, "invalid invite code")
		return
	}

	user, err := h.service.CreateUser(req.Username, req.Password, req.PIN, req.Email, false)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	if err := h.service.AddUserToDrive(user.ID.String(), invite.DriveID.String(), "", ""); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]interface{}{
		"id":       user.ID.String(),
		"username": user.Username,
		"email":    user.Email,
	})
}

func (h *AuthHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "missing user context")
		return
	}

	user, err := h.service.Store.GetUserByID(userID)
	if err != nil {
		response.NotFoundResponse(w, "user not found")
		return
	}

	inviteCode, err := h.service.InviteUser(user)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]string{"invite_code": inviteCode})
}

func (h *AuthHandler) RemoveUserFromDrive(w http.ResponseWriter, r *http.Request) {
	type removeUserRequest struct {
		UserID    string `json:"user_id"`
		DriveID   string `json:"drive_id"`
		Username  string `json:"username"`
		DriveSlug string `json:"drive_slug"`
	}
	var req removeUserRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	if err := h.service.RemoveUserFromDrive(req.UserID, req.DriveID, req.Username, req.DriveSlug); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]string{"message": "user removed from drive"})
}

func (h *AuthHandler) AddUserToDrive(w http.ResponseWriter, r *http.Request) {
	type addUserRequest struct {
		UserID    string `json:"user_id"`
		DriveID   string `json:"drive_id"`
		Username  string `json:"username"`
		DriveSlug string `json:"drive_slug"`
	}
	var req addUserRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	if err := h.service.AddUserToDrive(req.UserID, req.DriveID, req.Username, req.DriveSlug); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]string{"message": "user added to drive"})
}

func (h *AuthHandler) GetUsersInDrive(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	users, err := h.service.GetUsersInDrive(userId)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, map[string]interface{}{"users": users})
}

func (h *AuthHandler) GetUserInfoHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	userInfo, err := h.service.GetUserInfo(userId)
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	response.JSONResponse(w, userInfo)
}
