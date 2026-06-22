package admin

import (
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	announcements *service.AnnouncementService
	audit         *service.AuditService
}

func NewAnnouncementHandler(announcements *service.AnnouncementService, audit *service.AuditService) *AnnouncementHandler {
	return &AnnouncementHandler{announcements: announcements, audit: audit}
}

func (h *AnnouncementHandler) List(c *gin.Context) {
	items, err := h.announcements.List(service.AnnouncementListFilter{
		Keyword:    c.Query("keyword"),
		Status:     c.Query("status"),
		NotifyMode: c.Query("notifyMode"),
		Limit:      limit(c),
		Offset:     offset(c),
	})
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, items)
}

func (h *AnnouncementHandler) Create(c *gin.Context) {
	var req service.AnnouncementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid announcement payload")
		return
	}
	item, err := h.announcements.Create(req, adminID(c))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "announcement.create", "announcement", stringID(item.ID), c.ClientIP(), c.Request.UserAgent(), gin.H{
		"title":      item.Title,
		"status":     item.Status,
		"notifyMode": item.NotifyMode,
	})
	response.Created(c, item)
}

func (h *AnnouncementHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req service.AnnouncementInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid announcement payload")
		return
	}
	item, err := h.announcements.Update(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "announcement.update", "announcement", stringID(item.ID), c.ClientIP(), c.Request.UserAgent(), gin.H{
		"title":      item.Title,
		"status":     item.Status,
		"notifyMode": item.NotifyMode,
	})
	response.OK(c, item)
}

func (h *AnnouncementHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.announcements.Delete(uint(id)); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "announcement.delete", "announcement", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), gin.H{})
	response.OK(c, gin.H{"deleted": true})
}
