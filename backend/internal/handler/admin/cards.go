package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type CardHandler struct {
	cards *service.CardService
	audit *service.AuditService
}

func NewCardHandler(cards *service.CardService, audit *service.AuditService) *CardHandler {
	return &CardHandler{cards: cards, audit: audit}
}

func (h *CardHandler) CreateBatch(c *gin.Context) {
	var req service.CreateBatchInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid card batch payload")
		return
	}
	batch, err := h.cards.CreateBatch(req, adminID(c))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card_batch.create", "card_batch", stringID(batch.ID), c.ClientIP(), c.Request.UserAgent(), req)
	response.Created(c, batch)
}

func (h *CardHandler) ListBatches(c *gin.Context) {
	batches, err := h.cards.ListBatches(limit(c), offset(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, batches)
}

func (h *CardHandler) ExportBatch(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	content, err := h.cards.ExportBatch(uint(id))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card_batch.export", "card_batch", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=card-batch-%d.txt", id))
	c.String(http.StatusOK, content)
}

func (h *CardHandler) DeleteBatch(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.cards.DeleteBatch(uint(id)); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card_batch.delete", "card_batch", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, gin.H{"deleted": true})
}

func (h *CardHandler) ListCards(c *gin.Context) {
	cards, err := h.cards.ListCards(limit(c), offset(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, cards)
}

func (h *CardHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "status is required")
		return
	}
	if err := h.cards.UpdateStatus(uint(id), req.Status); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card.update_status", "card", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), req)
	response.OK(c, gin.H{"id": id, "status": req.Status})
}

func (h *CardHandler) DeleteCard(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.cards.DeleteCard(uint(id)); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card.delete", "card", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, gin.H{"deleted": true})
}

func (h *CardHandler) RevealCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	code, err := h.cards.RevealCode(uint(id))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "card.reveal_code", "card", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, gin.H{"code": code})
}
