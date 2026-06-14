package public

import (
	"net/http"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orders *service.OrderService
}

func NewOrderHandler(orders *service.OrderService) *OrderHandler {
	return &OrderHandler{orders: orders}
}

func (h *OrderHandler) Create(c *gin.Context) {
	var req cardCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "cardCode is required")
		return
	}
	order, err := h.orders.Create(c.Request.Context(), req.CardCode)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.Created(c, order)
}

func (h *OrderHandler) Get(c *gin.Context) {
	cardCode := c.Query("card_code")
	if cardCode == "" {
		cardCode = c.Query("cardCode")
	}
	if cardCode == "" {
		response.BadRequest(c, "cardCode is required")
		return
	}
	order, err := h.orders.GetByCard(c.Request.Context(), c.Param("orderNo"), cardCode)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	var req cardCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "cardCode is required")
		return
	}
	order, err := h.orders.CancelByCard(c.Request.Context(), c.Param("orderNo"), req.CardCode)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, order)
}

func (h *OrderHandler) History(c *gin.Context) {
	cardCode := c.Query("card_code")
	if cardCode == "" {
		cardCode = c.Query("cardCode")
	}
	if cardCode == "" {
		response.BadRequest(c, "cardCode is required")
		return
	}
	orders, err := h.orders.History(cardCode, 50, 0)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, orders)
}
