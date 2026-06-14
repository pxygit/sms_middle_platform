package admin

import (
	"net/http"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	admins *service.AdminService
	audit  *service.AuditService
}

func NewAuthHandler(admins *service.AdminService, audit *service.AuditService) *AuthHandler {
	return &AuthHandler{admins: admins, audit: audit}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "username and password are required")
		return
	}
	result, err := h.admins.Login(req.Username, req.Password)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	h.audit.Record("admin", result.Admin.ID, "admin.login", "admin", stringID(result.Admin.ID), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, result)
}
