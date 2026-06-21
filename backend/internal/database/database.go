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
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
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
	); err != nil {
		return err
	}
	return dropReceiveOrderServiceConfigConstraint(db)
}

func dropReceiveOrderServiceConfigConstraint(db *gorm.DB) error {
	return db.Exec(`
DO $$
DECLARE
	item record;
BEGIN
	FOR item IN
		SELECT namespace.nspname AS schema_name, table_class.relname AS table_name, constraint_info.conname AS constraint_name
		FROM pg_constraint constraint_info
		JOIN pg_class table_class ON table_class.oid = constraint_info.conrelid
		JOIN pg_namespace namespace ON namespace.oid = table_class.relnamespace
		JOIN unnest(constraint_info.conkey) AS constraint_column(attnum) ON true
		JOIN pg_attribute attribute_info ON attribute_info.attrelid = constraint_info.conrelid AND attribute_info.attnum = constraint_column.attnum
		WHERE constraint_info.contype = 'f'
		  AND table_class.relname = 'sys_receive_orders'
		  AND attribute_info.attname = 'service_config_id'
	LOOP
		EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS %I', item.schema_name, item.table_name, item.constraint_name);
	END LOOP;
END $$;
`).Error
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
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "herosms",
		Name:         "HeroSMS",
		BaseURL:      cfg.HeroSMSBaseURL,
		CurrencyCode: "USD",
		APIKey:       cfg.HeroSMSAPIKey,
	}); err != nil {
		return err
	}
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "smsbower",
		Name:         "SMSBower",
		BaseURL:      cfg.SMSBowerBaseURL,
		CurrencyCode: "USD",
		APIKey:       cfg.SMSBowerAPIKey,
	}); err != nil {
		return err
	}
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "lubansms",
		Name:         "LubanSMS",
		BaseURL:      cfg.LubanSMSBaseURL,
		CurrencyCode: "USD",
		APIKey:       cfg.LubanSMSAPIKey,
	}); err != nil {
		return err
	}
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:            "68sms",
		Name:            "68SMS",
		BaseURL:         cfg.SMS68BaseURL,
		CurrencyCode:    "USD",
		APIKey:          cfg.SMS68APIKey,
		MetadataToken:   cfg.SMS68MetadataToken,
		LongLived:       true,
		LoginCredential: true,
	}); err != nil {
		return err
	}
	if err := seedProvider(db, cfg.DataEncryptionKey, providerSeed{
		Code:         "62-us",
		Name:         "\u4e00\u6d69\u7f8e\u56fd\u63a5\u7801",
		BaseURL:      cfg.SMS62USBaseURL,
		CurrencyCode: "USD",
		APIKey:       cfg.SMS62USAPIKey,
		LongLived:    true,
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
	Code            string
	Name            string
	BaseURL         string
	CurrencyCode    string
	APIKey          string
	MetadataToken   string
	LongLived       bool
	LoginCredential bool
}

func seedProvider(db *gorm.DB, encryptionKey string, seed providerSeed) error {
	var count int64
	if err := db.Model(&model.SMSProvider{}).Where("code = ?", seed.Code).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		capabilities, _ := json.Marshal(map[string]bool{
			"balance":          true,
			"pricing":          true,
			"stock":            true,
			"cancel":           !seed.LongLived,
			"polling":          !seed.LongLived,
			"manual_check":     seed.LongLived,
			"login_credential": seed.LoginCredential,
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
	if seed.APIKey == "" && seed.MetadataToken == "" && !seed.LoginCredential {
		return nil
	}
	var provider model.SMSProvider
	if err := db.Where("code = ?", seed.Code).First(&provider).Error; err != nil {
		return err
	}
	updates := map[string]interface{}{
		"base_url": seed.BaseURL,
	}
	if seed.LoginCredential {
		capabilities := map[string]bool{}
		if len(provider.Capabilities) > 0 {
			_ = json.Unmarshal(provider.Capabilities, &capabilities)
		}
		if !capabilities["login_credential"] {
			capabilities["balance"] = true
			capabilities["pricing"] = true
			capabilities["stock"] = true
			capabilities["cancel"] = !seed.LongLived
			capabilities["polling"] = !seed.LongLived
			capabilities["manual_check"] = seed.LongLived
			capabilities["login_credential"] = true
			raw, _ := json.Marshal(capabilities)
			updates["capabilities"] = raw
		}
	}
	if seed.APIKey != "" && provider.APIKeyCipher == "" {
		cipherText, err := util.EncryptString(encryptionKey, seed.APIKey)
		if err != nil {
			return err
		}
		updates["api_key_cipher"] = cipherText
	}
	if seed.MetadataToken != "" && provider.MetadataTokenCipher == "" {
		cipherText, err := util.EncryptString(encryptionKey, seed.MetadataToken)
		if err != nil {
			return err
		}
		updates["metadata_token_cipher"] = cipherText
	}
	if len(updates) == 1 {
		return nil
	}
	return db.Model(&provider).Updates(updates).Error
}
