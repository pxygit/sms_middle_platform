package admin

import (
	"net/http"
	"strconv"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/response"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct {
	catalog *service.CatalogService
	meta    *service.ProviderMetadataService
	audit   *service.AuditService
}

func NewCatalogHandler(catalog *service.CatalogService, meta *service.ProviderMetadataService, audit *service.AuditService) *CatalogHandler {
	return &CatalogHandler{catalog: catalog, meta: meta, audit: audit}
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

func (h *CatalogHandler) DeleteServiceConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := h.catalog.DeleteServiceConfig(uint(id)); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.audit.Record("admin", adminID(c), "service_config.delete", "service_config", c.Param("id"), c.ClientIP(), c.Request.UserAgent(), nil)
	response.OK(c, gin.H{"deleted": true})
}

func (h *CatalogHandler) ProviderCountries(c *gin.Context) {
	countries, err := h.meta.Countries(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, countries)
}

func (h *CatalogHandler) ProviderServices(c *gin.Context) {
	services, err := h.meta.Services(c.Request.Context(), c.Param("provider"), c.Query("countryId"))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, services)
}

func (h *CatalogHandler) ProviderPrice(c *gin.Context) {
	price, err := h.meta.Price(c.Request.Context(), c.Param("provider"), sms.ProviderPriceInput{
		CountryID: c.Query("countryId"),
		ServiceID: c.Query("serviceId"),
		PoolID:    c.Query("poolId"),
	})
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, price)
}

func (h *CatalogHandler) ProviderStock(c *gin.Context) {
	stock, err := h.meta.Stock(c.Request.Context(), c.Param("provider"), sms.ProviderStockInput{
		CountryID: c.Query("countryId"),
		ServiceID: c.Query("serviceId"),
		PoolID:    c.Query("poolId"),
	})
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, stock)
}

func (h *CatalogHandler) ProviderBalance(c *gin.Context) {
	balance, err := h.meta.Balance(c.Request.Context(), c.Param("provider"))
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.OK(c, balance)
}
