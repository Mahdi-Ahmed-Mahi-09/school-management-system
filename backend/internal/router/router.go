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
	TeacherAssignments  *service.TeacherAssignmentService
	Attendance          *service.AttendanceService
	Examinations        *service.ExaminationService
	Results             *service.ResultService
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
	assignH := handler.NewTeacherAssignmentHandler(d.TeacherAssignments)
	attendanceH := handler.NewAttendanceHandler(d.Attendance)
	examH := handler.NewExaminationHandler(d.Examinations)
	resultH := handler.NewResultHandler(d.Results)
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
	students.GET("/:id/attendance", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.StudentAttendance)
	students.GET("/:id/attendance/summary", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.StudentAttendanceSummary)

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
	teachers.GET("/:id/assignments", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.TeacherAssignments)

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
	classes.GET("/:id/teacher-assignments", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.ClassTeachers)
	classes.POST("", middleware.RequirePermission(models.PermClassesCreate), classH.CreateClass)
	classes.PUT("/order", middleware.RequirePermission(models.PermClassesUpdate), classH.ReorderClasses)
	classes.PUT("/:id", middleware.RequirePermission(models.PermClassesUpdate), classH.UpdateClass)
	classes.PATCH("/:id/status", middleware.RequirePermission(models.PermClassesUpdate), classH.SetClassStatus)
	classes.DELETE("/:id", middleware.RequirePermission(models.PermClassesDelete), classH.DeleteClass)

	sections := protected.Group("/sections", middleware.RequireRoles(models.RoleAdmin))
	sections.GET("", middleware.RequirePermission(models.PermSectionsRead), classH.ListSections)
	sections.GET("/:id", middleware.RequirePermission(models.PermSectionsRead), classH.GetSection)
	sections.GET("/:id/teacher-assignments", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.SectionTeachers)
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

	// Teacher assignments: admin role AND the specific permission are both
	// required. Changing the teacher and (de)activating use the update permission.
	assignments := protected.Group("/teacher-assignments", middleware.RequireRoles(models.RoleAdmin))
	assignments.GET("", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.ListAssignments)
	assignments.GET("/stats", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.AssignmentStats)
	assignments.GET("/:id", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.GetAssignment)
	assignments.POST("", middleware.RequirePermission(models.PermTeacherAssignmentsCreate), assignH.CreateAssignment)
	assignments.POST("/bulk", middleware.RequirePermission(models.PermTeacherAssignmentsCreate), assignH.BulkAssign)
	assignments.PUT("/:id", middleware.RequirePermission(models.PermTeacherAssignmentsUpdate), assignH.UpdateAssignment)
	assignments.PATCH("/:id/status", middleware.RequirePermission(models.PermTeacherAssignmentsUpdate), assignH.SetAssignmentStatus)
	assignments.DELETE("/:id", middleware.RequirePermission(models.PermTeacherAssignmentsDelete), assignH.DeleteAssignment)

	// Class-teacher views of the same assignments.
	classTeachers := protected.Group("/class-teacher-assignments", middleware.RequireRoles(models.RoleAdmin))
	classTeachers.GET("", middleware.RequirePermission(models.PermTeacherAssignmentsRead), assignH.ListClassTeachers)
	classTeachers.POST("", middleware.RequirePermission(models.PermTeacherAssignmentsCreate), assignH.CreateClassTeacher)
	classTeachers.PUT("/:id", middleware.RequirePermission(models.PermTeacherAssignmentsUpdate), assignH.UpdateAssignment)

	// Attendance: admins and teachers. Teachers are further limited by the
	// service to the sections of their active assignments, and to today.
	attendance := protected.Group("/attendance", middleware.RequireRoles(models.RoleAdmin, models.RoleTeacher))
	attendance.GET("/options", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.Options)
	attendance.GET("", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.List)
	attendance.GET("/session", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.Session)
	attendance.GET("/session/summary", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.Summary)
	attendance.GET("/:id", middleware.RequirePermission(models.PermAttendanceRead), attendanceH.Get)
	attendance.POST("", middleware.RequirePermission(models.PermAttendanceMark), attendanceH.Mark)
	attendance.POST("/bulk", middleware.RequirePermission(models.PermAttendanceMark), attendanceH.SaveSession)
	attendance.PUT("/:id", middleware.RequirePermission(models.PermAttendanceMark), attendanceH.Update)
	attendance.PATCH("/:id/status", middleware.RequirePermission(models.PermAttendanceMark), attendanceH.SetStatus)

	// Reports and CSV export: admin role AND the specific permission.
	attendanceAdmin := protected.Group("/attendance", middleware.RequireRoles(models.RoleAdmin))
	attendanceAdmin.GET("/reports/daily", middleware.RequirePermission(models.PermAttendanceReports), attendanceH.DailyReport)
	attendanceAdmin.GET("/reports/monthly", middleware.RequirePermission(models.PermAttendanceReports), attendanceH.MonthlyReport)
	attendanceAdmin.GET("/reports/class-wise", middleware.RequirePermission(models.PermAttendanceReports), attendanceH.ClassWiseReport)
	attendanceAdmin.GET("/reports/student-wise", middleware.RequirePermission(models.PermAttendanceReports), attendanceH.StudentWiseReport)
	attendanceAdmin.GET("/export", middleware.RequirePermission(models.PermAttendanceExport), attendanceH.Export)

	// Examinations: admin role AND the specific permission are both required.
	// Update covers classes and schedules; publish covers publish/complete/cancel.
	exams := protected.Group("/examinations", middleware.RequireRoles(models.RoleAdmin))
	exams.GET("", middleware.RequirePermission(models.PermExamsRead), examH.List)
	exams.GET("/statistics", middleware.RequirePermission(models.PermExamsRead), examH.Stats)
	exams.GET("/:id", middleware.RequirePermission(models.PermExamsRead), examH.Get)
	exams.GET("/:id/history", middleware.RequirePermission(models.PermExamsRead), examH.History)
	exams.GET("/:id/classes", middleware.RequirePermission(models.PermExamsRead), examH.Classes)
	exams.GET("/:id/schedules", middleware.RequirePermission(models.PermExamsRead), examH.Classes)
	exams.GET("/:id/publish-check", middleware.RequirePermission(models.PermExamsPublish), examH.PublishCheck)
	exams.POST("", middleware.RequirePermission(models.PermExamsCreate), examH.Create)
	exams.PUT("/:id", middleware.RequirePermission(models.PermExamsUpdate), examH.Update)
	exams.DELETE("/:id", middleware.RequirePermission(models.PermExamsDelete), examH.Delete)
	exams.POST("/:id/classes", middleware.RequirePermission(models.PermExamsUpdate), examH.AddClasses)
	exams.DELETE("/:id/classes/:classId", middleware.RequirePermission(models.PermExamsUpdate), examH.RemoveClass)
	exams.POST("/:id/schedules", middleware.RequirePermission(models.PermExamsUpdate), examH.AddSchedules)
	exams.PUT("/:id/classes/:classId/schedules", middleware.RequirePermission(models.PermExamsUpdate), examH.ReplaceClassSchedule)
	exams.PATCH("/:id/publish", middleware.RequirePermission(models.PermExamsPublish), examH.Publish)
	exams.PATCH("/:id/complete", middleware.RequirePermission(models.PermExamsPublish), examH.Complete)
	exams.PATCH("/:id/cancel", middleware.RequirePermission(models.PermExamsPublish), examH.Cancel)

	examSchedules := protected.Group("/examination-schedules", middleware.RequireRoles(models.RoleAdmin))
	examSchedules.PUT("/:id", middleware.RequirePermission(models.PermExamsUpdate), examH.UpdateSchedule)
	examSchedules.DELETE("/:id", middleware.RequirePermission(models.PermExamsUpdate), examH.DeleteSchedule)

	examTypes := protected.Group("/exam-types", middleware.RequireRoles(models.RoleAdmin))
	examTypes.GET("", middleware.RequirePermission(models.PermExamsRead), examH.Types)
	examTypes.POST("", middleware.RequirePermission(models.PermExamsCreate), examH.CreateType)
	examTypes.PUT("/:id", middleware.RequirePermission(models.PermExamsUpdate), examH.UpdateType)
	examTypes.DELETE("/:id", middleware.RequirePermission(models.PermExamsDelete), examH.DeleteType)

	// Timetables: read-only for admins and teachers. The service limits
	// teachers to published/past examinations of the classes they teach.
	timetables := protected.Group("", middleware.RequireRoles(models.RoleAdmin, models.RoleTeacher), middleware.RequirePermission(models.PermExamTimetablesRead))
	timetables.GET("/exam-timetables", examH.Timetables)
	timetables.GET("/examinations/:id/timetable", examH.Timetable)
	timetables.GET("/examinations/:id/classes/:classId/timetable", examH.ClassTimetable)

	// Marks entry: admins and teachers. The service limits teachers to the
	// subjects and sections they teach (Feature 09) and to drafts.
	marks := protected.Group("", middleware.RequireRoles(models.RoleAdmin, models.RoleTeacher), middleware.RequirePermission(models.PermMarksEnter))
	marks.GET("/mark-entry/examinations", resultH.MarkExams)
	marks.GET("/examinations/:id/mark-sheets", resultH.Sheets)
	marks.GET("/examination-schedules/:id/mark-configuration", resultH.Scheme)
	marks.GET("/examination-schedules/:id/marks", resultH.Sheet)
	marks.POST("/examination-schedules/:id/marks", resultH.SaveMark)
	marks.POST("/examination-schedules/:id/marks/bulk", resultH.SaveMarks)
	marks.POST("/examination-schedules/:id/marks/submit", resultH.SheetAction((*service.ResultService).Submit))
	marks.PUT("/examination-marks/:id", resultH.UpdateMark)
	marks.GET("/examination-marks/:id/history", resultH.MarkHistory)

	// Marking schemes, verification, processing, publication and reports:
	// admin role AND the specific permission are both required.
	resultsAdmin := protected.Group("", middleware.RequireRoles(models.RoleAdmin))
	resultsAdmin.GET("/examinations/:id/mark-configurations", middleware.RequirePermission(models.PermMarksConfigure), resultH.Schemes)
	resultsAdmin.POST("/examination-schedules/:id/mark-configuration", middleware.RequirePermission(models.PermMarksConfigure), resultH.SaveScheme)
	resultsAdmin.PUT("/examination-schedules/:id/mark-configuration", middleware.RequirePermission(models.PermMarksConfigure), resultH.SaveScheme)
	resultsAdmin.GET("/examinations/:id/marks-review", middleware.RequirePermission(models.PermMarksVerify), resultH.Sheets)
	resultsAdmin.PATCH("/examination-schedules/:id/marks/verify", middleware.RequirePermission(models.PermMarksVerify), resultH.SheetAction((*service.ResultService).Verify))
	resultsAdmin.PATCH("/examination-schedules/:id/marks/finalize", middleware.RequirePermission(models.PermMarksVerify), resultH.SheetAction((*service.ResultService).Finalize))
	resultsAdmin.PATCH("/examination-schedules/:id/marks/reopen", middleware.RequirePermission(models.PermMarksVerify), resultH.SheetAction((*service.ResultService).Reopen))
	resultsAdmin.POST("/examinations/:id/process-subject-results", middleware.RequirePermission(models.PermResultsProcess), resultH.Step((*service.ResultService).ProcessSubjectResults))
	resultsAdmin.POST("/examinations/:id/process-overall-results", middleware.RequirePermission(models.PermResultsProcess), resultH.Step((*service.ResultService).ProcessOverallResults))
	resultsAdmin.PATCH("/examinations/:id/verify-results", middleware.RequirePermission(models.PermResultsProcess), resultH.Step((*service.ResultService).VerifyResults))
	resultsAdmin.POST("/examinations/:id/publish-results", middleware.RequirePermission(models.PermResultsPublish), resultH.Step((*service.ResultService).PublishResults))
	resultsAdmin.POST("/examinations/:id/correct-published-results", middleware.RequirePermission(models.PermResultsPublish), resultH.Correct)
	resultsAdmin.GET("/examinations/:id/result-status", middleware.RequirePermission(models.PermResultsRead), resultH.Status)
	resultsAdmin.GET("/examinations/:id/result-statistics", middleware.RequirePermission(models.PermResultsRead), resultH.Status)
	resultsAdmin.GET("/examinations/:id/results", middleware.RequirePermission(models.PermResultsRead), resultH.Results)
	resultsAdmin.GET("/examinations/:id/classes/:classId/results", middleware.RequirePermission(models.PermResultsRead), resultH.ClassResults)
	resultsAdmin.GET("/examinations/:id/merit-list", middleware.RequirePermission(models.PermResultsRead), resultH.MeritList)
	resultsAdmin.GET("/examinations/:id/classes/:classId/merit-list", middleware.RequirePermission(models.PermResultsRead), resultH.ClassMeritList)
	resultsAdmin.GET("/examinations/:id/students/:studentId/report-card", middleware.RequirePermission(models.PermResultsRead), resultH.ReportCard)
	resultsAdmin.GET("/examination-results/:id", middleware.RequirePermission(models.PermResultsRead), resultH.Result)
	resultsAdmin.GET("/students/:id/examination-results", middleware.RequirePermission(models.PermResultsRead), resultH.StudentResults)

	return r
}
