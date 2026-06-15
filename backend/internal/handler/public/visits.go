package public

import (
	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type VisitHandler struct {
	dashboard *service.DashboardService
}

func NewVisitHandler(dashboard *service.DashboardService) *VisitHandler {
	return &VisitHandler{dashboard: dashboard}
}

func (h *VisitHandler) Record(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.dashboard.RecordVisit(req.Path, c.ClientIP(), c.Request.UserAgent()); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"recorded": true})
}
