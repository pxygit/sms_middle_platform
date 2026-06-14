package router

import (
	"time"

	"sms-middle-platform/backend/internal/config"
	adminhandler "sms-middle-platform/backend/internal/handler/admin"
	publichandler "sms-middle-platform/backend/internal/handler/public"
	"sms-middle-platform/backend/internal/middleware"
	"sms-middle-platform/backend/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Services struct {
	Admins  *service.AdminService
	Audit   *service.AuditService
	Catalog *service.CatalogService
	Meta    *service.ProviderMetadataService
	Cards   *service.CardService
	Orders  *service.OrderService
}

func New(cfg config.Config, services Services) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	publicAPI := v1.Group("/public", middleware.RateLimit(120, time.Minute))
	{
		cardHandler := publichandler.NewCardHandler(services.Cards)
		orderHandler := publichandler.NewOrderHandler(services.Orders)
		publicAPI.POST("/cards/verify", cardHandler.Verify)
		publicAPI.POST("/orders", orderHandler.Create)
		publicAPI.GET("/orders/:orderNo", orderHandler.Get)
		publicAPI.POST("/orders/:orderNo/cancel", orderHandler.Cancel)
		publicAPI.GET("/cards/history", orderHandler.History)
	}

	adminAPI := v1.Group("/admin")
	{
		authHandler := adminhandler.NewAuthHandler(services.Admins, services.Audit)
		adminAPI.POST("/auth/login", middleware.RateLimit(20, time.Minute), authHandler.Login)

		protected := adminAPI.Group("", middleware.AdminAuth(cfg.JWTSecret))
		catalogHandler := adminhandler.NewCatalogHandler(services.Catalog, services.Meta, services.Audit)
		cardHandler := adminhandler.NewCardHandler(services.Cards, services.Audit)
		orderHandler := adminhandler.NewOrderHandler(services.Orders, services.Audit)
		auditHandler := adminhandler.NewAuditHandler(services.Audit)

		protected.POST("/auth/password", authHandler.ChangePassword)
		protected.GET("/providers", catalogHandler.Providers)
		protected.GET("/providers/:provider/countries", catalogHandler.ProviderCountries)
		protected.GET("/providers/:provider/services", catalogHandler.ProviderServices)
		protected.GET("/providers/:provider/price", catalogHandler.ProviderPrice)
		protected.GET("/providers/:provider/stock", catalogHandler.ProviderStock)
		protected.GET("/service-configs", catalogHandler.ListServiceConfigs)
		protected.POST("/service-configs", catalogHandler.CreateServiceConfig)
		protected.PATCH("/service-configs/:id", catalogHandler.UpdateServiceConfig)
		protected.DELETE("/service-configs/:id", catalogHandler.DeleteServiceConfig)
		protected.GET("/card-batches", cardHandler.ListBatches)
		protected.POST("/card-batches", cardHandler.CreateBatch)
		protected.GET("/card-batches/:id/export.txt", cardHandler.ExportBatch)
		protected.GET("/card-codes", cardHandler.ListCards)
		protected.GET("/card-codes/:id/reveal", cardHandler.RevealCode)
		protected.PATCH("/card-codes/:id/status", cardHandler.UpdateStatus)
		protected.GET("/orders", orderHandler.List)
		protected.POST("/orders/:id/cancel", orderHandler.Cancel)
		protected.GET("/audit-logs", auditHandler.List)
	}

	return r
}
