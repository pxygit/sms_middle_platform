package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/adapter/sms/firefox"
	"sms-middle-platform/backend/internal/adapter/sms/herosms"
	"sms-middle-platform/backend/internal/adapter/sms/lubansms"
	"sms-middle-platform/backend/internal/adapter/sms/sms62us"
	"sms-middle-platform/backend/internal/adapter/sms/sms68"
	"sms-middle-platform/backend/internal/adapter/sms/smsbower"
	"sms-middle-platform/backend/internal/adapter/sms/smspool"
	"sms-middle-platform/backend/internal/config"
	"sms-middle-platform/backend/internal/database"
	"sms-middle-platform/backend/internal/job"
	"sms-middle-platform/backend/internal/router"
	"sms-middle-platform/backend/internal/service"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := database.SeedDefaults(db, cfg); err != nil {
		log.Fatalf("seed defaults: %v", err)
	}

	registry := sms.NewRegistry()
	supplierLogs := service.NewSupplierLogService(db)
	registry.Register(smspool.New(cfg.SMSPoolAPIKey, cfg.SMSPoolBaseURL, cfg.SMSPoolTimeout, supplierLogs.Record))
	registry.Register(firefox.New(cfg.FirefoxAPIKey, cfg.FirefoxBaseURL, cfg.FirefoxTimeout, supplierLogs.Record))
	registry.Register(herosms.New(cfg.HeroSMSAPIKey, cfg.HeroSMSBaseURL, cfg.HeroSMSTimeout, supplierLogs.Record))
	registry.Register(smsbower.New(cfg.SMSBowerAPIKey, cfg.SMSBowerBaseURL, cfg.SMSBowerTimeout, supplierLogs.Record))
	registry.Register(lubansms.New(cfg.LubanSMSAPIKey, cfg.LubanSMSBaseURL, cfg.LubanSMSTimeout, supplierLogs.Record))
	registry.Register(sms68.New(cfg.SMS68APIKey, cfg.SMS68BaseURL, cfg.SMS68MetadataToken, cfg.SMS68Timeout, supplierLogs.Record))
	registry.Register(sms62us.New(cfg.SMS62USAPIKey, cfg.SMS62USBaseURL, cfg.SMS62USTimeout, supplierLogs.Record))

	admins := service.NewAdminService(db, cfg)
	audit := service.NewAuditService(db)
	catalog := service.NewCatalogService(db, cfg.DataEncryptionKey, registry)
	if err := catalog.ConfigureRuntimeProviders(); err != nil {
		log.Fatalf("configure providers: %v", err)
	}
	meta := service.NewProviderMetadataService(db, registry)
	cards := service.NewCardService(db, cfg.CardExportDir, cfg.DataEncryptionKey)
	orders := service.NewOrderService(db, registry)
	dashboard := service.NewDashboardService(db)
	announcements := service.NewAnnouncementService(db)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	job.NewPoller(orders, cfg.OrderPollInterval).Start(ctx)
	job.NewProviderMetadataSyncer(meta).Start(ctx)

	engine := router.New(cfg, router.Services{
		Admins:        admins,
		Audit:         audit,
		Catalog:       catalog,
		Meta:          meta,
		Cards:         cards,
		Orders:        orders,
		Dashboard:     dashboard,
		Announcements: announcements,
	})
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
