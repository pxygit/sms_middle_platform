package public

import (
	"net/http"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type CardHandler struct {
	cards *service.CardService
}

func NewCardHandler(cards *service.CardService) *CardHandler {
	return &CardHandler{cards: cards}
}

type cardCodeRequest struct {
	CardCode string `json:"cardCode" binding:"required"`
}

func (h *CardHandler) Verify(c *gin.Context) {
	var req cardCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "cardCode is required")
		return
	}
	result, err := h.cards.Verify(req.CardCode)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, result)
}
