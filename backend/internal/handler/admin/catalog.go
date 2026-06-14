package admin

import (
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct {
	catalog *service.CatalogService
	audit   *service.AuditService
}

func NewCatalogHandler(catalog *service.CatalogService, audit *service.AuditService) *CatalogHandler {
	return &CatalogHandler{catalog: catalog, audit: audit}
}

func (h *CatalogHandler) Providers(c *gin.Context) {
	providers, err := h.catalog.Providers()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, providers)
}

func (h *CatalogHandler) ListServiceConfigs(c *gin.Context) {
	configs, err := h.catalog.ListServiceConfigs()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, configs)
}

func (h *CatalogHandler) CreateServiceConfig(c *gin.Context) {
	var req service.ServiceConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid service config payload")
		return
	}
	config, err := h.catalog.CreateServiceConfig(req)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "service_config.create", "service_config", stringID(config.ID), c.ClientIP(), c.Request.UserAgent(), req)
	response.Created(c, config)
}

func (h *CatalogHandler) UpdateServiceConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req service.ServiceConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid service config payload")
		return
	}
	config, err := h.catalog.UpdateServiceConfig(uint(id), req)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "service_config.update", "service_config", stringID(config.ID), c.ClientIP(), c.Request.UserAgent(), req)
	response.OK(c, config)
}
