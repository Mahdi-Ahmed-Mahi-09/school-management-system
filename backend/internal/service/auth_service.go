// Package service implements Classora's business logic.
package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// Error codes returned by authentication.
const (
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeAccountInactive    = "ACCOUNT_INACTIVE"
	CodeAuthRequired       = "AUTH_REQUIRED"
	CodeSessionExpired     = "SESSION_EXPIRED"
	CodeSessionInvalid     = "SESSION_INVALID"
)

var (
	errInvalidCredentials = apperror.Unauthorized(CodeInvalidCredentials,
		"The email/username or password you entered is incorrect.")
	errSessionInvalid = apperror.Unauthorized(CodeSessionInvalid,
		"Your session is no longer valid. Please sign in again.")
)

// Session is an authenticated user together with their signed token.
type Session struct {
	User   *models.User
	Token  string
	Claims *auth.Claims
}

// ExpiresAt returns when the session token expires.
func (s *Session) ExpiresAt() time.Time { return s.Claims.ExpiresAt.Time }

// LoginInput is the login request body.
type LoginInput struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// AuthService handles sign-in, sign-out and token verification.
type AuthService struct {
	users      repository.UserRepository
	tokens     repository.TokenRepository
	tokenMgr   *auth.TokenManager
	bcryptCost int
}

// NewAuthService creates an AuthService.
func NewAuthService(users repository.UserRepository, tokens repository.TokenRepository, tm *auth.TokenManager, bcryptCost int) *AuthService {
	return &AuthService{users: users, tokens: tokens, tokenMgr: tm, bcryptCost: bcryptCost}
}

// Login verifies credentials and issues a session token.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*Session, error) {
	identifier := strings.TrimSpace(in.Identifier)
	errs := validation.Errors{}
	if identifier == "" {
		errs.Add("identifier", "Enter your email or username.")
	} else if len(identifier) > validation.MaxEmailLength {
		errs.Add("identifier", "Email or username is too long.")
	}
	if in.Password == "" {
		errs.Add("password", "Enter your password.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}

	user, err := s.users.GetByIdentifier(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		auth.EqualizeTiming(in.Password, s.bcryptCost)
		return nil, errInvalidCredentials
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !auth.CheckPassword(user.PasswordHash, in.Password) {
		return nil, errInvalidCredentials
	}
	// Only revealed after a correct password, so it cannot be used for enumeration.
	if !user.IsActive() {
		return nil, apperror.Forbidden("Your account has been deactivated. Please contact your school administrator.").
			WithCode(CodeAccountInactive)
	}

	if err := s.users.TouchLastLogin(ctx, user.ID); err != nil {
		slog.Warn("could not record last login", "user_id", user.ID, "error", err)
	} else {
		now := time.Now()
		user.LastLoginAt = &now
	}
	return s.IssueSession(user, in.RememberMe)
}

// IssueSession signs a fresh token for an already-authenticated user.
func (s *AuthService) IssueSession(user *models.User, remember bool) (*Session, error) {
	token, claims, err := s.tokenMgr.Issue(user.ID, string(user.Role), user.TokenVersion, remember)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &Session{User: user, Token: token, Claims: claims}, nil
}

// Authenticate validates a token and returns the current state of its user.
func (s *AuthService) Authenticate(ctx context.Context, token string) (*models.User, *auth.Claims, error) {
	if token == "" {
		return nil, nil, apperror.Unauthorized(CodeAuthRequired, "Please sign in to continue.")
	}
	claims, err := s.tokenMgr.Parse(token)
	if errors.Is(err, auth.ErrTokenExpired) {
		return nil, nil, apperror.Unauthorized(CodeSessionExpired, "Your session has expired. Please sign in again.")
	}
	if err != nil {
		return nil, nil, errSessionInvalid
	}

	jti, _ := claims.TokenID()
	revoked, err := s.tokens.IsRevoked(ctx, jti)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	if revoked {
		return nil, nil, errSessionInvalid
	}

	userID, _ := claims.UserID()
	user, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil, errSessionInvalid
	}
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	if !user.IsActive() {
		return nil, nil, apperror.Unauthorized(CodeAccountInactive,
			"Your account has been deactivated. Please contact your school administrator.")
	}
	// A password change or reset bumps token_version, invalidating older tokens.
	if claims.Version != user.TokenVersion {
		return nil, nil, errSessionInvalid
	}
	return user, claims, nil
}

// Logout revokes the token so it cannot be reused.
func (s *AuthService) Logout(ctx context.Context, claims *auth.Claims) error {
	jti, err := claims.TokenID()
	if err != nil {
		return apperror.BadRequest("Invalid session.")
	}
	userID, err := claims.UserID()
	if err != nil {
		return apperror.BadRequest("Invalid session.")
	}
	if err := s.tokens.Revoke(ctx, jti, userID, claims.ExpiresAt.Time); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// PurgeExpiredRevocations removes revocation records that are no longer needed.
func (s *AuthService) PurgeExpiredRevocations(ctx context.Context) (int64, error) {
	return s.tokens.DeleteExpired(ctx)
}
