package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/middleware"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// UserHandler serves the admin user-management endpoints.
type UserHandler struct {
	users *service.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(users *service.UserService) *UserHandler {
	return &UserHandler{users: users}
}

func queryInt(c *gin.Context, errs validation.Errors, key string) int {
	raw := c.Query(key)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		errs.Add(key, "Must be a positive whole number.")
		return 0
	}
	return n
}

// List handles GET /users.
func (h *UserHandler) List(c *gin.Context) {
	errs := validation.Errors{}
	in := service.ListUsersInput{
		Search:    c.Query("search"),
		Role:      c.Query("role"),
		Status:    c.Query("status"),
		SortBy:    c.Query("sort"),
		SortOrder: c.Query("order"),
		Page:      queryInt(c, errs, "page"),
		PageSize:  queryInt(c, errs, "page_size"),
	}
	if errs.Any() {
		response.Error(c, apperror.Validation(errs))
		return
	}
	result, err := h.users.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Paginated(c, models.ToDTOs(result.Users),
		response.NewPagination(result.Page, result.PageSize, result.Total))
}

// Stats handles GET /users/stats.
func (h *UserHandler) Stats(c *gin.Context) {
	stats, err := h.users.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// Get handles GET /users/:id.
func (h *UserHandler) Get(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	user, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user.ToDTO())
}

// Create handles POST /users.
func (h *UserHandler) Create(c *gin.Context) {
	var in service.CreateUserInput
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	user, err := h.users.Create(c.Request.Context(), middleware.CurrentUser(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, user.ToDTO())
}

// withTarget parses the :id parameter and JSON body, then runs fn.
func withTarget[T any](c *gin.Context, fn func(actor *models.User, id uuid.UUID, in T) (*models.User, error)) {
	id, err := pathUUID(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var in T
	if err := bindJSON(c, &in); err != nil {
		response.Error(c, err)
		return
	}
	user, err := fn(middleware.CurrentUser(c), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user.ToDTO())
}

// Update handles PATCH /users/:id.
func (h *UserHandler) Update(c *gin.Context) {
	withTarget(c, func(actor *models.User, id uuid.UUID, in service.UpdateUserInput) (*models.User, error) {
		return h.users.Update(c.Request.Context(), actor, id, in)
	})
}

// ChangeRole handles PATCH /users/:id/role.
func (h *UserHandler) ChangeRole(c *gin.Context) {
	withTarget(c, func(actor *models.User, id uuid.UUID, in service.ChangeRoleInput) (*models.User, error) {
		return h.users.ChangeRole(c.Request.Context(), actor, id, in)
	})
}

// ChangeStatus handles PATCH /users/:id/status.
func (h *UserHandler) ChangeStatus(c *gin.Context) {
	withTarget(c, func(actor *models.User, id uuid.UUID, in service.ChangeStatusInput) (*models.User, error) {
		return h.users.ChangeStatus(c.Request.Context(), actor, id, in)
	})
}

// ResetPassword handles PUT /users/:id/password.
func (h *UserHandler) ResetPassword(c *gin.Context) {
	withTarget(c, func(actor *models.User, id uuid.UUID, in service.ResetPasswordInput) (*models.User, error) {
		return h.users.ResetPassword(c.Request.Context(), actor, id, in)
	})
}
