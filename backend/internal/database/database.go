package database

import (
	"encoding/json"

	"sms-middle-platform/backend/internal/config"
	"sms-middle-platform/backend/internal/model"
	"sms-middle-platform/backend/internal/util"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	logLevel := logger.Warn
	if cfg.AppEnv == "development" {
		logLevel = logger.Info
	}
	return gorm.Open(postgres.Open(cfg.DatabaseDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Admin{},
		&model.SMSProvider{},
		&model.ProviderCountry{},
		&model.ProviderService{},
		&model.ServiceConfig{},
		&model.CardBatch{},
		&model.CardCode{},
		&model.ReceiveOrder{},
		&model.SupplierRequestLog{},
		&model.AuditLog{},
		&model.SiteVisit{},
	)
}

func SeedDefaults(db *gorm.DB, cfg config.Config) error {
	var count int64
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "smspool",
		Name:         "SMSPool",
		BaseURL:      cfg.SMSPoolBaseURL,
		CurrencyCode: "USD",
		APIKey:       cfg.SMSPoolAPIKey,
	}); err != nil {
		return err
	}
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "firefox",
		Name:         "Firefox",
		BaseURL:      cfg.FirefoxBaseURL,
		CurrencyCode: "CNY",
		APIKey:       cfg.FirefoxAPIKey,
	}); err != nil {
		return err
	}

	if err := db.Model(&model.Admin{}).Where("username = ?", cfg.AdminDefaultUsername).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminDefaultPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		return db.Create(&model.Admin{
			Username:     cfg.AdminDefaultUsername,
			PasswordHash: string(hash),
			Role:         "super_admin",
			Status:       model.StatusEnabled,
		}).Error
	}
	return nil
}

type providerSeed struct {
	Code         string
	Name         string
	BaseURL      string
	CurrencyCode string
	APIKey       string
}

func seedProvider(db *gorm.DB, encryptionKey string, seed providerSeed) error {
	var count int64
	if err := db.Model(&model.SMSProvider{}).Where("code = ?", seed.Code).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		capabilities, _ := json.Marshal(map[string]bool{
			"balance": true,
			"pricing": true,
			"stock":   true,
			"cancel":  true,
			"polling": true,
		})
		if err := db.Create(&model.SMSProvider{
			Code:         seed.Code,
			Name:         seed.Name,
			BaseURL:      seed.BaseURL,
			CurrencyCode: seed.CurrencyCode,
			Status:       model.StatusEnabled,
			Capabilities: capabilities,
		}).Error; err != nil {
			return err
		}
	}
	if seed.APIKey == "" {
		return nil
	}
	var provider model.SMSProvider
	if err := db.Where("code = ?", seed.Code).First(&provider).Error; err != nil {
		return err
	}
	if provider.APIKeyCipher != "" {
		return nil
	}
	cipherText, err := util.EncryptString(encryptionKey, seed.APIKey)
	if err != nil {
		return err
	}
	return db.Model(&provider).Updates(map[string]interface{}{
		"api_key_cipher": cipherText,
		"base_url":       seed.BaseURL,
	}).Error
}
