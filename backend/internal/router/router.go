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
	AuthService         *service.AuthService
	UserService         *service.UserService
	DashboardService    *service.DashboardService
	StudentService      *service.StudentService
	TeacherService      *service.TeacherService
	AcademicYearService *service.AcademicYearService
	ClassService        *service.ClassService
	SectionService      *service.SectionService
	SubjectService      *service.SubjectService
	ClassSubjectService *service.ClassSubjectService
	EnrollmentService   *service.EnrollmentService
	PromotionService    *service.PromotionService
	Cookie              handler.CookieConfig
	AllowedOrigins      []string
	LoginRateLimit      int
	LoginRateWindow     time.Duration
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
	yearH := handler.NewAcademicYearHandler(d.AcademicYearService)
	classH := handler.NewClassHandler(d.ClassService, d.SectionService)
	subjectH := handler.NewSubjectHandler(d.SubjectService, d.ClassSubjectService)
	enrollH := handler.NewEnrollmentHandler(d.EnrollmentService, d.PromotionService)
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
	students.GET("/:id/enrollment-history", middleware.RequirePermission(models.PermEnrollmentsRead), enrollH.StudentEnrollmentHistory)

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

	// Academic years: admin role AND the specific permission are both required.
	years := protected.Group("/academic-years", middleware.RequireRoles(models.RoleAdmin))
	years.GET("", middleware.RequirePermission(models.PermAcademicYearsRead), yearH.List)
	years.GET("/stats", middleware.RequirePermission(models.PermAcademicYearsRead), yearH.Stats)
	years.GET("/:id", middleware.RequirePermission(models.PermAcademicYearsRead), yearH.Get)
	years.POST("", middleware.RequirePermission(models.PermAcademicYearsCreate), yearH.Create)
	years.PUT("/:id", middleware.RequirePermission(models.PermAcademicYearsUpdate), yearH.Update)
	years.PATCH("/:id/activate", middleware.RequirePermission(models.PermAcademicYearsManageStatus), yearH.Activate)
	years.PATCH("/:id/complete", middleware.RequirePermission(models.PermAcademicYearsManageStatus), yearH.Complete)
	years.DELETE("/:id", middleware.RequirePermission(models.PermAcademicYearsDelete), yearH.Delete)

	// Classes and sections: admin role AND the specific permission are both
	// required. Status changes and ordering use the update permission.
	classes := protected.Group("/classes", middleware.RequireRoles(models.RoleAdmin))
	classes.GET("", middleware.RequirePermission(models.PermClassesRead), classH.ListClasses)
	classes.GET("/stats", middleware.RequirePermission(models.PermClassesRead), classH.ClassStats)
	classes.GET("/:id", middleware.RequirePermission(models.PermClassesRead), classH.GetClass)
	classes.GET("/:id/sections", middleware.RequirePermission(models.PermSectionsRead), classH.ClassSections)
	classes.GET("/:id/subjects", middleware.RequirePermission(models.PermSubjectsRead), subjectH.ClassSubjects)
	classes.POST("", middleware.RequirePermission(models.PermClassesCreate), classH.CreateClass)
	classes.PUT("/order", middleware.RequirePermission(models.PermClassesUpdate), classH.ReorderClasses)
	classes.PUT("/:id", middleware.RequirePermission(models.PermClassesUpdate), classH.UpdateClass)
	classes.PATCH("/:id/status", middleware.RequirePermission(models.PermClassesUpdate), classH.SetClassStatus)
	classes.DELETE("/:id", middleware.RequirePermission(models.PermClassesDelete), classH.DeleteClass)

	sections := protected.Group("/sections", middleware.RequireRoles(models.RoleAdmin))
	sections.GET("", middleware.RequirePermission(models.PermSectionsRead), classH.ListSections)
	sections.GET("/:id", middleware.RequirePermission(models.PermSectionsRead), classH.GetSection)
	sections.POST("", middleware.RequirePermission(models.PermSectionsCreate), classH.CreateSection)
	sections.PUT("/:id", middleware.RequirePermission(models.PermSectionsUpdate), classH.UpdateSection)
	sections.PATCH("/:id/status", middleware.RequirePermission(models.PermSectionsUpdate), classH.SetSectionStatus)
	sections.DELETE("/:id", middleware.RequirePermission(models.PermSectionsDelete), classH.DeleteSection)

	// Subjects and class-subject assignments: admin role AND the specific
	// permission are both required. Status changes use the update permission;
	// adding, editing and removing assignments use subjects:assign.
	subjects := protected.Group("/subjects", middleware.RequireRoles(models.RoleAdmin))
	subjects.GET("", middleware.RequirePermission(models.PermSubjectsRead), subjectH.ListSubjects)
	subjects.GET("/stats", middleware.RequirePermission(models.PermSubjectsRead), subjectH.SubjectStats)
	subjects.GET("/:id", middleware.RequirePermission(models.PermSubjectsRead), subjectH.GetSubject)
	subjects.GET("/:id/classes", middleware.RequirePermission(models.PermSubjectsRead), subjectH.SubjectClasses)
	subjects.POST("", middleware.RequirePermission(models.PermSubjectsCreate), subjectH.CreateSubject)
	subjects.PUT("/:id", middleware.RequirePermission(models.PermSubjectsUpdate), subjectH.UpdateSubject)
	subjects.PATCH("/:id/status", middleware.RequirePermission(models.PermSubjectsUpdate), subjectH.SetSubjectStatus)
	subjects.DELETE("/:id", middleware.RequirePermission(models.PermSubjectsDelete), subjectH.DeleteSubject)

	classSubjects := protected.Group("/class-subjects", middleware.RequireRoles(models.RoleAdmin))
	classSubjects.GET("", middleware.RequirePermission(models.PermSubjectsRead), subjectH.ListAssignments)
	classSubjects.GET("/:id", middleware.RequirePermission(models.PermSubjectsRead), subjectH.GetAssignment)
	classSubjects.POST("", middleware.RequirePermission(models.PermSubjectsAssign), subjectH.AssignSubjects)
	classSubjects.PUT("/:id", middleware.RequirePermission(models.PermSubjectsAssign), subjectH.UpdateAssignment)
	classSubjects.DELETE("/:id", middleware.RequirePermission(models.PermSubjectsAssign), subjectH.DeleteAssignment)

	// Enrollments and promotions: admin role AND the specific permission are
	// both required. Withdrawing/re-admitting uses the update permission.
	enrollments := protected.Group("/enrollments", middleware.RequireRoles(models.RoleAdmin))
	enrollments.GET("", middleware.RequirePermission(models.PermEnrollmentsRead), enrollH.ListEnrollments)
	enrollments.GET("/stats", middleware.RequirePermission(models.PermEnrollmentsRead), enrollH.EnrollmentStats)
	enrollments.GET("/unenrolled-students", middleware.RequirePermission(models.PermEnrollmentsCreate), enrollH.UnenrolledStudents)
	enrollments.GET("/:id", middleware.RequirePermission(models.PermEnrollmentsRead), enrollH.GetEnrollment)
	enrollments.POST("", middleware.RequirePermission(models.PermEnrollmentsCreate), enrollH.CreateEnrollment)
	enrollments.POST("/bulk", middleware.RequirePermission(models.PermEnrollmentsCreate), enrollH.BulkEnroll)
	enrollments.PUT("/:id", middleware.RequirePermission(models.PermEnrollmentsUpdate), enrollH.UpdateEnrollment)
	enrollments.PATCH("/:id/status", middleware.RequirePermission(models.PermEnrollmentsUpdate), enrollH.SetEnrollmentStatus)
	enrollments.DELETE("/:id", middleware.RequirePermission(models.PermEnrollmentsDelete), enrollH.DeleteEnrollment)

	promotions := protected.Group("/promotions", middleware.RequireRoles(models.RoleAdmin))
	promotions.GET("", middleware.RequirePermission(models.PermPromotionsRead), enrollH.ListPromotions)
	promotions.GET("/:id", middleware.RequirePermission(models.PermPromotionsRead), enrollH.GetPromotion)
	promotions.POST("/preview", middleware.RequirePermission(models.PermPromotionsExecute), enrollH.PreviewPromotion)
	promotions.POST("/execute", middleware.RequirePermission(models.PermPromotionsExecute), enrollH.ExecutePromotion)

	return r
}
