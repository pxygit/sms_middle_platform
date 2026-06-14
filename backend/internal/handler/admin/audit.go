package admin

import (
	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	audit *service.AuditService
}

func NewAuditHandler(audit *service.AuditService) *AuditHandler {
	return &AuditHandler{audit: audit}
}

func (h *AuditHandler) List(c *gin.Context) {
	logs, err := h.audit.List(limit(c), offset(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, logs)
}
