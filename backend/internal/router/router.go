// Package router wires middleware and handlers into the Gin engine.
package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/handler"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Deps are the collaborators the router needs.
type Deps struct {
	AuthService     *service.AuthService
	UserService     *service.UserService
	Cookie          handler.CookieConfig
	AllowedOrigins  []string
	LoginRateLimit  int
	LoginRateWindow time.Duration
	// HealthCheck verifies dependencies such as the database. Optional.
	HealthCheck func(ctx context.Context) error
}

// New builds the HTTP handler for the API.
func New(d Deps) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.HandleMethodNotAllowed = true

	r.Use(
		gin.CustomRecovery(func(c *gin.Context, rec any) {
			response.Error(c, apperror.Internal(fmt.Errorf("panic: %v", rec)))
		}),
		middleware.RequestLogger(slog.Info),
		middleware.SecurityHeaders(),
		middleware.CORS(d.AllowedOrigins),
		middleware.BodyLimit(maxBodyBytes),
	)
	r.NoRoute(func(c *gin.Context) { response.Error(c, apperror.NotFound("Endpoint not found.")) })
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, &apperror.Error{Status: http.StatusMethodNotAllowed, Code: "METHOD_NOT_ALLOWED", Message: "Method not allowed."})
	})

	authH := handler.NewAuthHandler(d.AuthService, d.Cookie)
	profileH := handler.NewProfileHandler(d.UserService, d.AuthService, authH)
	userH := handler.NewUserHandler(d.UserService)
	loginLimiter := middleware.NewRateLimiter(d.LoginRateLimit, d.LoginRateWindow)

	api := r.Group("/api/v1", middleware.CSRFGuard())

	api.GET("/health", func(c *gin.Context) {
		if d.HealthCheck != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := d.HealthCheck(ctx); err != nil {
				response.Error(c, &apperror.Error{Status: http.StatusServiceUnavailable, Code: "UNAVAILABLE", Message: "Service unavailable.", Err: err})
				return
			}
		}
		response.OK(c, gin.H{"status": "ok"})
	})

	// Public authentication endpoints.
	api.POST("/auth/login", loginLimiter.Middleware(), authH.Login)
	api.POST("/auth/logout", authH.Logout)

	// Everything below requires a valid session.
	protected := api.Group("", middleware.Authenticate(d.AuthService, d.Cookie.Name))
	protected.GET("/auth/me", authH.Me)
	protected.GET("/roles", authH.Roles)

	profile := protected.Group("/profile")
	profile.GET("", middleware.RequirePermission(models.PermProfileRead), profileH.Get)
	profile.PATCH("", middleware.RequirePermission(models.PermProfileUpdate), profileH.Update)
	profile.PUT("/password", middleware.RequirePermission(models.PermPasswordChange), profileH.ChangePassword)

	// User administration: admin role AND the specific permission are both required.
	users := protected.Group("/users", middleware.RequireRoles(models.RoleAdmin))
	users.GET("", middleware.RequirePermission(models.PermUsersRead), userH.List)
	users.GET("/stats", middleware.RequirePermission(models.PermUsersRead), userH.Stats)
	users.GET("/:id", middleware.RequirePermission(models.PermUsersRead), userH.Get)
	users.POST("", middleware.RequirePermission(models.PermUsersCreate), userH.Create)
	users.PATCH("/:id", middleware.RequirePermission(models.PermUsersUpdate), userH.Update)
	users.PATCH("/:id/role", middleware.RequirePermission(models.PermUsersManageRoles), userH.ChangeRole)
	users.PATCH("/:id/status", middleware.RequirePermission(models.PermUsersManageStatus), userH.ChangeStatus)
	users.PUT("/:id/password", middleware.RequirePermission(models.PermUsersResetPassword), userH.ResetPassword)

	return r
}
