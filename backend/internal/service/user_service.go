package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/auth"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	DefaultPageSize = 10
	MaxPageSize     = 100
	maxSearchLength = 100
)

var errUserNotFound = apperror.NotFound("User not found.")

// ListUsersInput holds raw listing parameters from the query string.
type ListUsersInput struct {
	Search    string
	Role      string
	Status    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

// ListUsersResult is a page of users.
type ListUsersResult struct {
	Users    []models.User
	Total    int
	Page     int
	PageSize int
}

// CreateUserInput is the admin "create user" request body.
type CreateUserInput struct {
	Username           string `json:"username"`
	Email              string `json:"email"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Phone              string `json:"phone"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	Password           string `json:"password"`
	MustChangePassword bool   `json:"must_change_password"`
}

// UpdateUserInput is the admin "edit user" body. Omitted fields are unchanged.
type UpdateUserInput struct {
	Username  *string `json:"username"`
	Email     *string `json:"email"`
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Phone     *string `json:"phone"`
}

// UpdateProfileInput contains the only fields users may change about themselves.
type UpdateProfileInput struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Phone     *string `json:"phone"`
}

// ChangePasswordInput is the self-service password change body.
type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// ResetPasswordInput is the admin password reset body.
type ResetPasswordInput struct {
	NewPassword        string `json:"new_password"`
	MustChangePassword bool   `json:"must_change_password"`
}

// ChangeRoleInput is the admin role assignment body.
type ChangeRoleInput struct {
	Role string `json:"role"`
}

// ChangeStatusInput is the admin activation body.
type ChangeStatusInput struct {
	Status string `json:"status"`
}

// UserService implements user administration and self-service profile logic.
type UserService struct {
	users      repository.UserRepository
	bcryptCost int
}

// NewUserService creates a UserService.
func NewUserService(users repository.UserRepository, bcryptCost int) *UserService {
	return &UserService{users: users, bcryptCost: bcryptCost}
}

func mapRepoError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return errUserNotFound
	case errors.Is(err, repository.ErrDuplicateEmail):
		return apperror.Conflict("EMAIL_TAKEN", "That email address is already in use.",
			map[string]string{"email": "That email address is already in use."})
	case errors.Is(err, repository.ErrDuplicateUsername):
		return apperror.Conflict("USERNAME_TAKEN", "That username is already taken.",
			map[string]string{"username": "That username is already taken."})
	case errors.Is(err, repository.ErrLastAdmin):
		return apperror.Conflict("LAST_ADMIN",
			"This is the last active administrator. Assign another admin first.", nil)
	default:
		return apperror.Internal(err)
	}
}

// List returns a filtered page of users.
func (s *UserService) List(ctx context.Context, in ListUsersInput) (*ListUsersResult, error) {
	errs := validation.Errors{}
	filter := repository.UserFilter{
		Search:   strings.TrimSpace(in.Search),
		Page:     in.Page,
		PageSize: in.PageSize,
		SortBy:   in.SortBy,
	}
	if len([]rune(filter.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Role != "" {
		role, ok := models.ParseRole(in.Role)
		if !ok {
			errs.Add("role", "Unknown role.")
		}
		filter.Role = &role
	}
	if in.Status != "" {
		status, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		filter.Status = &status
	}
	switch filter.SortBy {
	case "":
		filter.SortBy = repository.SortCreatedAt
	case repository.SortCreatedAt, repository.SortName, repository.SortEmail, repository.SortRole, repository.SortLastLogin:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "", "desc":
		filter.SortDesc = in.SortOrder != "" || filter.SortBy == repository.SortCreatedAt
	case "asc":
		filter.SortDesc = false
	default:
		errs.Add("order", "Order must be asc or desc.")
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = DefaultPageSize
	}
	if filter.PageSize > MaxPageSize {
		filter.PageSize = MaxPageSize
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}

	users, total, err := s.users.List(ctx, filter)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListUsersResult{Users: users, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

// Stats returns account counts for the admin dashboard.
func (s *UserService) Stats(ctx context.Context) (*models.UserStats, error) {
	stats, err := s.users.Stats(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return stats, nil
}

// Get returns a single user.
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*models.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return u, nil
}

// Create validates input and creates a new account.
func (s *UserService) Create(ctx context.Context, actor *models.User, in CreateUserInput) (*models.User, error) {
	u := &models.User{
		Username:           validation.NormalizeUsername(in.Username),
		Email:              validation.NormalizeEmail(in.Email),
		FirstName:          validation.NormalizeName(in.FirstName),
		LastName:           validation.NormalizeName(in.LastName),
		Phone:              normalizePhone(in.Phone),
		MustChangePassword: in.MustChangePassword,
		CreatedBy:          &actor.ID,
	}

	errs := validation.Errors{}
	validateDetails(errs, u)

	role, ok := models.ParseRole(in.Role)
	if !ok {
		errs.Add("role", "Choose a valid role.")
	}
	u.Role = role

	u.Status = models.StatusActive
	if in.Status != "" {
		status, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose a valid status.")
		}
		u.Status = status
	}
	validation.Password(errs, "password", in.Password)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}

	hash, err := auth.HashPassword(in.Password, s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	u.PasswordHash = hash

	created, err := s.users.Create(ctx, u)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return created, nil
}

// Update edits a user's account details (admin only).
func (s *UserService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateUserInput) (*models.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}
	if in.Username != nil {
		u.Username = validation.NormalizeUsername(*in.Username)
	}
	if in.Email != nil {
		u.Email = validation.NormalizeEmail(*in.Email)
	}
	applyPersonalDetails(u, in.FirstName, in.LastName, in.Phone)

	errs := validation.Errors{}
	validateDetails(errs, u)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	updated, err := s.users.UpdateDetails(ctx, u, actor.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

// ChangeRole assigns a new role (admin only). Admins cannot change their own role.
func (s *UserService) ChangeRole(ctx context.Context, actor *models.User, id uuid.UUID, in ChangeRoleInput) (*models.User, error) {
	role, ok := models.ParseRole(in.Role)
	if !ok {
		return nil, apperror.Validation(map[string]string{"role": "Choose a valid role."})
	}
	if actor.ID == id {
		return nil, apperror.Forbidden("You can't change your own role.")
	}
	updated, err := s.users.UpdateRole(ctx, id, role, actor.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

// ChangeStatus activates or deactivates an account (admin only).
func (s *UserService) ChangeStatus(ctx context.Context, actor *models.User, id uuid.UUID, in ChangeStatusInput) (*models.User, error) {
	status, ok := models.ParseStatus(in.Status)
	if !ok {
		return nil, apperror.Validation(map[string]string{"status": "Choose a valid status."})
	}
	if actor.ID == id {
		return nil, apperror.Forbidden("You can't change the status of your own account.")
	}
	updated, err := s.users.UpdateStatus(ctx, id, status, actor.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

// ResetPassword sets a new password for another user and signs them out everywhere.
func (s *UserService) ResetPassword(ctx context.Context, actor *models.User, id uuid.UUID, in ResetPasswordInput) (*models.User, error) {
	if actor.ID == id {
		return nil, apperror.Forbidden("Use your profile page to change your own password.")
	}
	errs := validation.Errors{}
	validation.Password(errs, "new_password", in.NewPassword)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if _, err := s.users.GetByID(ctx, id); err != nil {
		return nil, mapRepoError(err)
	}
	hash, err := auth.HashPassword(in.NewPassword, s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.users.UpdatePassword(ctx, id, hash, in.MustChangePassword, actor.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

// UpdateProfile lets a user edit their own personal details.
// Role, status, email and username are deliberately not editable here.
func (s *UserService) UpdateProfile(ctx context.Context, user *models.User, in UpdateProfileInput) (*models.User, error) {
	u := *user
	applyPersonalDetails(&u, in.FirstName, in.LastName, in.Phone)

	errs := validation.Errors{}
	validateDetails(errs, &u)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	updated, err := s.users.UpdateDetails(ctx, &u, user.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

// ChangePassword lets a user change their own password after confirming the current one.
func (s *UserService) ChangePassword(ctx context.Context, user *models.User, in ChangePasswordInput) (*models.User, error) {
	errs := validation.Errors{}
	if in.CurrentPassword == "" {
		errs.Add("current_password", "Enter your current password.")
	}
	validation.Password(errs, "new_password", in.NewPassword)
	if in.ConfirmPassword != in.NewPassword {
		errs.Add("confirm_password", "Passwords do not match.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if !auth.CheckPassword(user.PasswordHash, in.CurrentPassword) {
		return nil, apperror.Validation(map[string]string{"current_password": "Your current password is incorrect."})
	}
	if in.CurrentPassword == in.NewPassword {
		return nil, apperror.Validation(map[string]string{"new_password": "Choose a password different from your current one."})
	}

	hash, err := auth.HashPassword(in.NewPassword, s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	updated, err := s.users.UpdatePassword(ctx, user.ID, hash, false, user.ID)
	if err != nil {
		return nil, mapRepoError(err)
	}
	return updated, nil
}

func applyPersonalDetails(u *models.User, first, last, phone *string) {
	if first != nil {
		u.FirstName = validation.NormalizeName(*first)
	}
	if last != nil {
		u.LastName = validation.NormalizeName(*last)
	}
	if phone != nil {
		u.Phone = normalizePhone(*phone)
	}
}

func validateDetails(errs validation.Errors, u *models.User) {
	validation.Username(errs, "username", u.Username)
	validation.Email(errs, "email", u.Email)
	validation.Name(errs, "first_name", "First name", u.FirstName)
	validation.Name(errs, "last_name", "Last name", u.LastName)
	if u.Phone != nil {
		validation.Phone(errs, "phone", *u.Phone)
	}
}

func normalizePhone(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
