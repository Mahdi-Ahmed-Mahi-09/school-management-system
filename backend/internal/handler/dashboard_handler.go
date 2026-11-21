package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/response"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/service"
)

// DashboardHandler serves dashboard endpoints.
type DashboardHandler struct {
	dashboard *service.DashboardService
}

// NewDashboardHandler creates a DashboardHandler.
func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

// Admin handles GET /admin/dashboard.
func (h *DashboardHandler) Admin(c *gin.Context) {
	data, err := h.dashboard.AdminDashboard(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, data)
}
