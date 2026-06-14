package service

import (
	"errors"

	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
)

type CatalogService struct {
	db *gorm.DB
}

type ServiceConfigInput struct {
	ProviderCode      string  `json:"providerCode" binding:"required"`
	TargetPlatform    string  `json:"targetPlatform" binding:"required"`
	DisplayName       string  `json:"displayName" binding:"required"`
	CountryCode       string  `json:"countryCode" binding:"required"`
	CountryName       string  `json:"countryName"`
	ProviderCountryID string  `json:"providerCountryId" binding:"required"`
	ProviderServiceID string  `json:"providerServiceId" binding:"required"`
	ProviderPoolID    string  `json:"providerPoolId"`
	MaxPrice          float64 `json:"maxPrice"`
	TimeoutSeconds    int     `json:"timeoutSeconds"`
	Status            string  `json:"status"`
}

func NewCatalogService(db *gorm.DB) *CatalogService {
	return &CatalogService{db: db}
}

func (s *CatalogService) Providers() ([]model.SMSProvider, error) {
	var providers []model.SMSProvider
	err := s.db.Order("id asc").Find(&providers).Error
	return providers, err
}

func (s *CatalogService) ListServiceConfigs() ([]model.ServiceConfig, error) {
	var configs []model.ServiceConfig
	err := s.db.Order("id desc").Find(&configs).Error
	return configs, err
}

func (s *CatalogService) CreateServiceConfig(input ServiceConfigInput) (*model.ServiceConfig, error) {
	if input.TimeoutSeconds <= 0 {
		input.TimeoutSeconds = 1200
	}
	if input.Status == "" {
		input.Status = model.StatusEnabled
	}
	config := model.ServiceConfig{
		ProviderCode:      input.ProviderCode,
		TargetPlatform:    input.TargetPlatform,
		DisplayName:       input.DisplayName,
		CountryCode:       input.CountryCode,
		CountryName:       input.CountryName,
		ProviderCountryID: input.ProviderCountryID,
		ProviderServiceID: input.ProviderServiceID,
		ProviderPoolID:    input.ProviderPoolID,
		MaxPrice:          input.MaxPrice,
		TimeoutSeconds:    input.TimeoutSeconds,
		Status:            input.Status,
	}
	return &config, s.db.Create(&config).Error
}

func (s *CatalogService) UpdateServiceConfig(id uint, input ServiceConfigInput) (*model.ServiceConfig, error) {
	var config model.ServiceConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"provider_code":       input.ProviderCode,
		"target_platform":     input.TargetPlatform,
		"display_name":        input.DisplayName,
		"country_code":        input.CountryCode,
		"country_name":        input.CountryName,
		"provider_country_id": input.ProviderCountryID,
		"provider_service_id": input.ProviderServiceID,
		"provider_pool_id":    input.ProviderPoolID,
		"max_price":           input.MaxPrice,
		"timeout_seconds":     input.TimeoutSeconds,
		"status":              input.Status,
	}
	if updates["timeout_seconds"].(int) <= 0 {
		updates["timeout_seconds"] = 1200
	}
	if updates["status"].(string) == "" {
		updates["status"] = model.StatusEnabled
	}
	if err := s.db.Model(&config).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &config, s.db.First(&config, id).Error
}

func (s *CatalogService) DeleteServiceConfig(id uint) error {
	var count int64
	if err := s.db.Model(&model.CardCode{}).Where("service_config_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("service config is already used by card codes")
	}
	if err := s.db.Model(&model.ReceiveOrder{}).Where("service_config_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("service config is already used by orders")
	}
	return s.db.Delete(&model.ServiceConfig{}, id).Error
}
