package admin

import (
	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	dashboard *service.DashboardService
}

func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Stats(c *gin.Context) {
	stats, err := h.dashboard.Stats()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, stats)
}
