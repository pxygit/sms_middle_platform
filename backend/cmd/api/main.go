package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"sms-middle-platform/backend/internal/adapter/sms"
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

	admins := service.NewAdminService(db, cfg)
	audit := service.NewAuditService(db)
	catalog := service.NewCatalogService(db)
	meta := service.NewProviderMetadataService(registry)
	cards := service.NewCardService(db, cfg.CardExportDir, cfg.DataEncryptionKey)
	orders := service.NewOrderService(db, registry)
	dashboard := service.NewDashboardService(db)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	job.NewPoller(orders, cfg.OrderPollInterval).Start(ctx)

	engine := router.New(cfg, router.Services{
		Admins:    admins,
		Audit:     audit,
		Catalog:   catalog,
		Meta:      meta,
		Cards:     cards,
		Orders:    orders,
		Dashboard: dashboard,
	})
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
