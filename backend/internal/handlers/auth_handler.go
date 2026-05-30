package handlers

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/auth"
	reqhelpers "archivus/pkg/reqHelpers"
	"archivus/pkg/response"
	"fmt"
	"net/http"
	"time"
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
	Username   string          `json:"username"`
	Password   string          `json:"password"`
	PIN        string          `json:"pin"`
	Email      string          `json:"email"`
	UserType   models.UserType `json:"user_type"`
	InviteCode string          `json:"invite_code"` // for business users
	IsAdmin    bool            `json:"is_admin"`    // for business users
	DriveName  string          `json:"drive_name"`  // for personal users
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

func registerPersonalUser(h *AuthHandler, req registerRequest) error {
	user, err := h.service.CreateUser(req.Username, req.Password, req.PIN, req.Email, req.UserType, true)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	_, err = h.service.SetupNewDrive(user.Username, user.ID.String())
	if err != nil {
		return fmt.Errorf("failed to create drive: %w", err)
	}
	return nil
}

func registerBusinessUser(h *AuthHandler, req registerRequest) error {
	if req.InviteCode == "" && !req.IsAdmin {
		return fmt.Errorf("invite code or admin flag required for business users")
	}
	var invite models.UserInvite
	var err error
	invite, err = h.service.ValidateInviteCode(req.InviteCode)
	if err != nil {
		return fmt.Errorf("invalid invite code: %w", err)
	}
	if invite.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("invite code has expired")
	}
	user, err := h.service.CreateUser(req.Username, req.Password, req.PIN, req.Email, req.UserType, req.IsAdmin)

	// business users can be added to drives after creation, if invite code is provided
	if req.InviteCode != "" {
		if err := h.service.AddUserToDrive(user.ID.String(), invite.DriveID.String(), "", ""); err != nil {
			return fmt.Errorf("failed to add user to drive: %w", err)
		}
		return nil
	}
	if req.DriveName != "" {
		_, err = h.service.SetupNewDrive(req.DriveName, user.ID.String())
	} else {
		_, err = h.service.SetupNewDrive(user.Username, user.ID.String())
	}
	if err != nil {
		return fmt.Errorf("failed to create drive: %w", err)
	}
	return nil
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}
	var err error
	if req.UserType == models.UserTypeBusiness {
		err = registerBusinessUser(h, req)
	} else if req.UserType == models.UserTypePersonal {
		err = registerPersonalUser(h, req)
	}
	if err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	response.JSONResponse(w, map[string]interface{}{
		"message": "user created",
	})
}

type inviteUserRequest struct {
	UserID  string             `json:"user_id"`
	DriveID string             `json:"drive_id"`
	Access  models.AccessLevel `json:"access"`
}

func (h *AuthHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(archivus_constants.ContextKey(archivus_constants.UserIdKey)).(string)
	if !ok {
		response.UnauthorizedResponse(w, "missing user context")
		return
	}
	var req inviteUserRequest
	if err := reqhelpers.DecodeRequest(r, &req); err != nil {
		response.BadRequestResponse(w, err.Error())
		return
	}

	user, err := h.service.Store.GetUserByID(userID)
	if err != nil {
		response.NotFoundResponse(w, "user not found")
		return
	}

	inviteCode, err := h.service.InviteUser(user, req.DriveID, req.Access)
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
