// Command server runs the Classora REST API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/config"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/database"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/handler"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/router"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		applied, err := database.MigrateUp(ctx, pool)
		if err != nil {
			return err
		}
		if len(applied) > 0 {
			slog.Info("applied migrations", "versions", applied)
		}
	}

	userRepo := repository.NewPostgresUserRepository(pool)
	tokenRepo := repository.NewPostgresTokenRepository(pool)
	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL, cfg.JWTRememberTTL)
	authSvc := service.NewAuthService(userRepo, tokenRepo, tokenMgr, cfg.BcryptCost)
	activityRepo := repository.NewPostgresActivityRepository(pool)
	userSvc := service.NewUserService(userRepo, activityRepo, cfg.BcryptCost)
	dashboardSvc := service.NewDashboardService(repository.NewPostgresDashboardRepository(pool), cfg.Timezone)
	studentSvc := service.NewStudentService(repository.NewPostgresStudentRepository(pool), activityRepo, cfg.Timezone)
	teacherSvc := service.NewTeacherService(repository.NewPostgresTeacherRepository(pool), activityRepo, cfg.Timezone, cfg.SalaryCurrency)
	academicYearSvc := service.NewAcademicYearService(repository.NewPostgresAcademicYearRepository(pool), activityRepo, cfg.Timezone)
	classRepo := repository.NewPostgresClassRepository(pool)
	classSvc := service.NewClassService(classRepo, activityRepo)
	sectionSvc := service.NewSectionService(repository.NewPostgresSectionRepository(pool), classRepo, activityRepo)
	subjectRepo := repository.NewPostgresSubjectRepository(pool)
	subjectSvc := service.NewSubjectService(subjectRepo, activityRepo)
	classSubjectSvc := service.NewClassSubjectService(repository.NewPostgresClassSubjectRepository(pool), classRepo, subjectRepo, activityRepo)

	engine := router.New(router.Deps{
		AuthService:         authSvc,
		UserService:         userSvc,
		DashboardService:    dashboardSvc,
		StudentService:      studentSvc,
		TeacherService:      teacherSvc,
		AcademicYearService: academicYearSvc,
		ClassService:        classSvc,
		SectionService:      sectionSvc,
		SubjectService:      subjectSvc,
		ClassSubjectService: classSubjectSvc,
		Cookie:              handler.CookieConfig{Name: cfg.CookieName, Secure: cfg.CookieSecure, Domain: cfg.CookieDomain},
		AllowedOrigins:      cfg.CORSAllowedOrigins,
		LoginRateLimit:      cfg.LoginRateLimit,
		LoginRateWindow:     cfg.LoginRateWindow,
		HealthCheck:         pool.Ping,
	})

	go purgeRevokedTokens(ctx, authSvc)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("classora api listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// purgeRevokedTokens periodically deletes revocation records of expired tokens.
func purgeRevokedTokens(ctx context.Context, authSvc *service.AuthService) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		if n, err := authSvc.PurgeExpiredRevocations(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("purge revoked tokens failed", "error", err)
		} else if n > 0 {
			slog.Info("purged expired revoked tokens", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
