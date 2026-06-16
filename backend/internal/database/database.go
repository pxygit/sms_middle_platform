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
	if err := db.Model(&model.SMSProvider{}).Where("code = ?", "smspool").Count(&count).Error; err != nil {
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
			Code:         "smspool",
			Name:         "SMSPool",
			BaseURL:      cfg.SMSPoolBaseURL,
			CurrencyCode: "USD",
			Status:       model.StatusEnabled,
			Capabilities: capabilities,
		}).Error; err != nil {
			return err
		}
	}
	if cfg.SMSPoolAPIKey != "" {
		var provider model.SMSProvider
		if err := db.Where("code = ?", "smspool").First(&provider).Error; err == nil && provider.APIKeyCipher == "" {
			cipherText, err := util.EncryptString(cfg.DataEncryptionKey, cfg.SMSPoolAPIKey)
			if err != nil {
				return err
			}
			if err := db.Model(&provider).Updates(map[string]interface{}{
				"api_key_cipher": cipherText,
				"base_url":       cfg.SMSPoolBaseURL,
			}).Error; err != nil {
				return err
			}
		}
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
