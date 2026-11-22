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

// maxBodyBytes caps every request body: 2 MiB leaves room for a 1 MiB photo
// plus multipart overhead; JSON endpoints are far smaller.
const maxBodyBytes = 2 << 20

// Deps are the collaborators the router needs.
type Deps struct {
	AuthService      *service.AuthService
	UserService      *service.UserService
	DashboardService *service.DashboardService
	StudentService   *service.StudentService
	TeacherService   *service.TeacherService
	Cookie           handler.CookieConfig
	AllowedOrigins   []string
	LoginRateLimit   int
	LoginRateWindow  time.Duration
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
	dashboardH := handler.NewDashboardHandler(d.DashboardService)
	studentH := handler.NewStudentHandler(d.StudentService)
	teacherH := handler.NewTeacherHandler(d.TeacherService)
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

	// Admin dashboard: admin role AND the dashboard permission are both required.
	admin := protected.Group("/admin", middleware.RequireRoles(models.RoleAdmin))
	admin.GET("/dashboard", middleware.RequirePermission(models.PermDashboardAdmin), dashboardH.Admin)

	// Student management: admin role AND the specific permission are both required.
	students := protected.Group("/students", middleware.RequireRoles(models.RoleAdmin))
	students.GET("", middleware.RequirePermission(models.PermStudentsRead), studentH.List)
	students.GET("/stats", middleware.RequirePermission(models.PermStudentsRead), studentH.Stats)
	students.GET("/:id", middleware.RequirePermission(models.PermStudentsRead), studentH.Get)
	students.GET("/:id/photo", middleware.RequirePermission(models.PermStudentsRead), studentH.Photo)
	students.POST("", middleware.RequirePermission(models.PermStudentsCreate), studentH.Create)
	students.PUT("/:id", middleware.RequirePermission(models.PermStudentsUpdate), studentH.Update)
	students.PUT("/:id/photo", middleware.RequirePermission(models.PermStudentsUpdate), studentH.UploadPhoto)
	students.DELETE("/:id/photo", middleware.RequirePermission(models.PermStudentsUpdate), studentH.RemovePhoto)
	students.DELETE("/:id", middleware.RequirePermission(models.PermStudentsDelete), studentH.Delete)

	// Teacher management: admin role AND the specific permission are both required.
	// Salary visibility is decided per field by the teachers:view_salary permission.
	teachers := protected.Group("/teachers", middleware.RequireRoles(models.RoleAdmin))
	teachers.GET("", middleware.RequirePermission(models.PermTeachersRead), teacherH.List)
	teachers.GET("/stats", middleware.RequirePermission(models.PermTeachersRead), teacherH.Stats)
	teachers.GET("/linkable-accounts", middleware.RequirePermission(models.PermTeachersUpdate), teacherH.LinkableAccounts)
	teachers.GET("/:id", middleware.RequirePermission(models.PermTeachersRead), teacherH.Get)
	teachers.GET("/:id/photo", middleware.RequirePermission(models.PermTeachersRead), teacherH.Photo)
	teachers.POST("", middleware.RequirePermission(models.PermTeachersCreate), teacherH.Create)
	teachers.PUT("/:id", middleware.RequirePermission(models.PermTeachersUpdate), teacherH.Update)
	teachers.PUT("/:id/photo", middleware.RequirePermission(models.PermTeachersUpdate), teacherH.UploadPhoto)
	teachers.DELETE("/:id/photo", middleware.RequirePermission(models.PermTeachersUpdate), teacherH.RemovePhoto)
	teachers.DELETE("/:id", middleware.RequirePermission(models.PermTeachersDelete), teacherH.Delete)

	return r
}
