package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

// ProfileHandler serves self-service profile endpoints.
type ProfileHandler struct {
	users *service.UserService
	auth  *service.AuthService
	authH *AuthHandler
}

// NewProfileHandler creates a ProfileHandler.
func NewProfileHandler(users *service.UserService, auth *service.AuthService, authH *AuthHandler) *ProfileHandler {
	return &ProfileHandler{users: users, auth: auth, authH: authH}
}

// Get handles GET /profile.
func (h *ProfileHandler) Get(c *gin.Context) {
	response.OK(c, middleware.CurrentUser(c).ToDTO())
}

// Update handles PATCH /profile.
func (h *ProfileHandler) Update(c *gin.Context) {
	var in service.UpdateProfileInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	updated, err := h.users.UpdateProfile(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, updated.ToDTO())
}

// ChangePassword handles PUT /profile/password.
//
// Changing the password invalidates every existing session (including other
// devices), so a fresh session is issued for the current browser.
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var in service.ChangePasswordInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	claims := middleware.CurrentClaims(c)
	updated, err := h.users.ChangePassword(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	session, err := h.auth.IssueSession(updated, claims.Remember)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.authH.setSessionCookie(c, session)
	response.OK(c, newAuthPayload(updated, session.ExpiresAt(), claims.Remember))
}
