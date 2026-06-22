package public

import (
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	announcements *service.AnnouncementService
}

func NewAnnouncementHandler(announcements *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements}
}

func (h *AnnouncementHandler) List(c *gin.Context) {
	items, err := h.announcements.PublicList(c.Query("readerId"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, items)
}

type readAnnouncementRequest struct {
	ReaderID string `json:"readerId" binding:"required"`
}

func (h *AnnouncementHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req readAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid reader payload")
		return
	}
	if err := h.announcements.MarkRead(uint(id), req.ReaderID, c.ClientIP(), c.Request.UserAgent()); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, gin.H{"read": true})
}
