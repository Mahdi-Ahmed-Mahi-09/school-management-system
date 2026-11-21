// Package middleware contains Gin middleware for security and access control.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

const (
	ctxUser   = "classora.user"
	ctxClaims = "classora.claims"
)

// Authenticate requires a valid session. The token is read from the
// HttpOnly session cookie, or from an "Authorization: Bearer" header for
// non-browser API clients.
func Authenticate(authSvc *service.AuthService, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c, cookieName)
		user, claims, err := authSvc.Authenticate(c.Request.Context(), token)
		if err != nil {
			response.Error(c, err)
			return
		}
		c.Set(ctxUser, user)
		c.Set(ctxClaims, claims)
		c.Next()
	}
}

func tokenFromRequest(c *gin.Context, cookieName string) string {
	if header := c.GetHeader("Authorization"); header != "" {
		if scheme, token, ok := strings.Cut(header, " "); ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(token)
		}
	}
	if cookie, err := c.Cookie(cookieName); err == nil {
		return cookie
	}
	return ""
}

// RequireRoles allows only users whose current role is one of roles.
func RequireRoles(roles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil {
			response.Error(c, apperror.Unauthorized(service.CodeAuthRequired, "Please sign in to continue."))
			return
		}
		for _, r := range roles {
			if user.Role == r {
				c.Next()
				return
			}
		}
		response.Error(c, apperror.Forbidden("You don't have permission to access this resource."))
	}
}

// RequirePermission allows only users whose role grants the permission.
func RequirePermission(p models.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil {
			response.Error(c, apperror.Unauthorized(service.CodeAuthRequired, "Please sign in to continue."))
			return
		}
		if !user.Role.Can(p) {
			response.Error(c, apperror.Forbidden("You don't have permission to perform this action."))
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user, or nil.
func CurrentUser(c *gin.Context) *models.User {
	if v, ok := c.Get(ctxUser); ok {
		if u, ok := v.(*models.User); ok {
			return u
		}
	}
	return nil
}

// CurrentClaims returns the authenticated token's claims, or nil.
func CurrentClaims(c *gin.Context) *auth.Claims {
	if v, ok := c.Get(ctxClaims); ok {
		if claims, ok := v.(*auth.Claims); ok {
			return claims
		}
	}
	return nil
}
