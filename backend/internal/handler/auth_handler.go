package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

// CookieConfig controls the session cookie.
type CookieConfig struct {
	Name   string
	Secure bool
	Domain string
}

// sessionCookiePath scopes the cookie to API requests only.
const sessionCookiePath = "/api"

// AuthHandler serves login, logout and current-user endpoints.
type AuthHandler struct {
	auth   *service.AuthService
	cookie CookieConfig
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(auth *service.AuthService, cookie CookieConfig) *AuthHandler {
	return &AuthHandler{auth: auth, cookie: cookie}
}

type sessionInfo struct {
	ExpiresAt  time.Time `json:"expires_at"`
	RememberMe bool      `json:"remember_me"`
}

type authPayload struct {
	User        models.UserDTO      `json:"user"`
	Permissions []models.Permission `json:"permissions"`
	Session     sessionInfo         `json:"session"`
}

func newAuthPayload(u *models.User, expiresAt time.Time, remember bool) authPayload {
	return authPayload{
		User:        u.ToDTO(),
		Permissions: models.PermissionsFor(u.Role),
		Session:     sessionInfo{ExpiresAt: expiresAt, RememberMe: remember},
	}
}

// setSessionCookie stores the JWT in an HttpOnly, SameSite=Strict cookie so
// it is never readable by JavaScript. "Remember me" makes it persistent;
// otherwise it is a browser-session cookie.
func (h *AuthHandler) setSessionCookie(c *gin.Context, s *service.Session) {
	cookie := &http.Cookie{
		Name:     h.cookie.Name,
		Value:    s.Token,
		Path:     sessionCookiePath,
		Domain:   h.cookie.Domain,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteStrictMode,
	}
	if s.Claims.Remember {
		cookie.Expires = s.ExpiresAt()
		cookie.MaxAge = int(time.Until(s.ExpiresAt()).Seconds())
	}
	http.SetCookie(c.Writer, cookie)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     h.cookie.Name,
		Value:    "",
		Path:     sessionCookiePath,
		Domain:   h.cookie.Domain,
		HttpOnly: true,
		Secure:   h.cookie.Secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var in service.LoginInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	session, err := h.auth.Login(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.setSessionCookie(c, session)
	response.OK(c, newAuthPayload(session.User, session.ExpiresAt(), session.Claims.Remember))
}

// Logout handles POST /auth/logout. It always clears the cookie, and revokes
// the token server-side when it is still valid.
func (h *AuthHandler) Logout(c *gin.Context) {
	if token, err := c.Cookie(h.cookie.Name); err == nil && token != "" {
		if _, claims, err := h.auth.Authenticate(c.Request.Context(), token); err == nil {
			if err := h.auth.Logout(c.Request.Context(), claims); err != nil {
				response.Error(c, err)
				return
			}
		}
	}
	h.clearSessionCookie(c)
	response.OK(c, gin.H{"message": "You have been signed out."})
}

// Me handles GET /auth/me.
func (h *AuthHandler) Me(c *gin.Context) {
	user := middleware.CurrentUser(c)
	claims := middleware.CurrentClaims(c)
	response.OK(c, newAuthPayload(user, claims.ExpiresAt.Time, claims.Remember))
}

// Roles handles GET /roles: every role with the permissions it grants.
func (h *AuthHandler) Roles(c *gin.Context) {
	response.OK(c, models.RoleDefinitions())
}
