package admin

import (
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	orders *service.OrderService
	audit  *service.AuditService
}

func NewOrderHandler(orders *service.OrderService, audit *service.AuditService) *OrderHandler {
	return &OrderHandler{orders: orders, audit: audit}
}

func (h *OrderHandler) List(c *gin.Context) {
	orders, err := h.orders.List(limit(c), offset(c))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, orders)
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	order, err := h.orders.Cancel(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "order.cancel", "order", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, order)
}
