package router

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	Admins        *service.AdminService
	Audit         *service.AuditService
	Catalog       *service.CatalogService
	Meta          *service.ProviderMetadataService
	Cards         *service.CardService
	Orders        *service.OrderService
	Dashboard     *service.DashboardService
	Announcements *service.AnnouncementService
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
		visitHandler := publichandler.NewVisitHandler(services.Dashboard)
		announcementHandler := publichandler.NewAnnouncementHandler(services.Announcements)
		publicAPI.POST("/cards/verify", cardHandler.Verify)
		publicAPI.POST("/orders", orderHandler.Create)
		publicAPI.GET("/orders/:orderNo", orderHandler.Get)
		publicAPI.POST("/orders/:orderNo/check", orderHandler.Check)
		publicAPI.POST("/orders/:orderNo/cancel", orderHandler.Cancel)
		publicAPI.GET("/cards/history", orderHandler.History)
		publicAPI.POST("/visits", visitHandler.Record)
		publicAPI.GET("/announcements", announcementHandler.List)
		publicAPI.POST("/announcements/:id/read", announcementHandler.MarkRead)
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
		dashboardHandler := adminhandler.NewDashboardHandler(services.Dashboard)
		announcementHandler := adminhandler.NewAnnouncementHandler(services.Announcements, services.Audit)

		protected.POST("/auth/password", authHandler.ChangePassword)
		protected.GET("/dashboard", dashboardHandler.Stats)
		protected.GET("/providers", catalogHandler.Providers)
		protected.PATCH("/providers/:provider", catalogHandler.UpdateProvider)
		protected.GET("/providers/:provider/countries", catalogHandler.ProviderCountries)
		protected.GET("/providers/:provider/services", catalogHandler.ProviderServices)
		protected.GET("/providers/:provider/price", catalogHandler.ProviderPrice)
		protected.GET("/providers/:provider/stock", catalogHandler.ProviderStock)
		protected.GET("/providers/:provider/quote", catalogHandler.ProviderQuote)
		protected.GET("/providers/:provider/validity-options", catalogHandler.ProviderValidityOptions)
		protected.GET("/providers/:provider/balance", catalogHandler.ProviderBalance)
		protected.GET("/service-configs", catalogHandler.ListServiceConfigs)
		protected.POST("/service-configs", catalogHandler.CreateServiceConfig)
		protected.PATCH("/service-configs/:id", catalogHandler.UpdateServiceConfig)
		protected.DELETE("/service-configs/:id", catalogHandler.DeleteServiceConfig)
		protected.GET("/card-batches", cardHandler.ListBatches)
		protected.POST("/card-batches", cardHandler.CreateBatch)
		protected.GET("/card-batches/:id/export.txt", cardHandler.ExportBatch)
		protected.DELETE("/card-batches/:id", cardHandler.DeleteBatch)
		protected.GET("/card-codes", cardHandler.ListCards)
		protected.GET("/card-codes/:id/reveal", cardHandler.RevealCode)
		protected.PATCH("/card-codes/:id/status", cardHandler.UpdateStatus)
		protected.DELETE("/card-codes/:id", cardHandler.DeleteCard)
		protected.GET("/orders", orderHandler.List)
		protected.POST("/orders/:id/cancel", orderHandler.Cancel)
		protected.GET("/audit-logs", auditHandler.List)
		protected.GET("/announcements", announcementHandler.List)
		protected.POST("/announcements", announcementHandler.Create)
		protected.PATCH("/announcements/:id", announcementHandler.Update)
		protected.DELETE("/announcements/:id", announcementHandler.Delete)
	}

	registerStatic(r, cfg.StaticDir)

	return r
}

// registerStatic serves the built frontend (SPA) directly from the API process
// when STATIC_DIR is set, so native deployments do not need a separate nginx.
func registerStatic(r *gin.Engine, staticDir string) {
	if staticDir == "" {
		return
	}
	root, err := filepath.Abs(staticDir)
	if err != nil {
		return
	}
	indexFile := filepath.Join(root, "index.html")

	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, "/api/") || requestPath == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "not found"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "message": "not found"})
			return
		}

		candidate := filepath.Join(root, filepath.FromSlash(path.Clean("/"+requestPath)))
		if strings.HasPrefix(candidate, root) {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				if strings.HasPrefix(requestPath, "/assets/") {
					// Vite emits content-hashed asset filenames; cache them aggressively.
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				c.File(candidate)
				return
			}
		}

		// SPA fallback: unknown paths render the frontend router.
		c.Header("Cache-Control", "no-cache")
		c.File(indexFile)
	})
}
