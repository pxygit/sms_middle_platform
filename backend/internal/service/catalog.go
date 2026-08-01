package service

import (
	"encoding/json"
	"errors"
	"strings"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"
	"sms-middle-platform/backend/internal/util"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CatalogService struct {
	db            *gorm.DB
	encryptionKey string
	registry      *sms.Registry
}

type ProviderInput struct {
	Name            string `json:"name" binding:"required"`
	BaseURL         string `json:"baseUrl"`
	CurrencyCode    string `json:"currencyCode"`
	APIKey          string `json:"apiKey"`
	LoginCredential string `json:"loginCredential"`
	MetadataToken   string `json:"metadataToken"`
	AuthMode        string `json:"authMode"`
	Account         string `json:"account"`
	Password        string `json:"password"`
	Status          string `json:"status"`
}

type ServiceConfigInput struct {
	ProviderCode      string         `json:"providerCode" binding:"required"`
	TargetPlatform    string         `json:"targetPlatform" binding:"required"`
	DisplayName       string         `json:"displayName" binding:"required"`
	CountryCode       string         `json:"countryCode" binding:"required"`
	CountryName       string         `json:"countryName"`
	ProviderCountryID string         `json:"providerCountryId" binding:"required"`
	ProviderServiceID string         `json:"providerServiceId" binding:"required"`
	ProviderPoolID    string         `json:"providerPoolId"`
	MaxPrice          float64        `json:"maxPrice"`
	TimeoutSeconds    int            `json:"timeoutSeconds"`
	Metadata          datatypes.JSON `json:"metadata"`
	Status            string         `json:"status"`
}

func NewCatalogService(db *gorm.DB, encryptionKey string, registry *sms.Registry) *CatalogService {
	return &CatalogService{db: db, encryptionKey: encryptionKey, registry: registry}
}

func (s *CatalogService) Providers() ([]model.SMSProvider, error) {
	var providers []model.SMSProvider
	err := s.db.Order("id asc").Find(&providers).Error
	for index := range providers {
		s.decorateProvider(&providers[index])
	}
	return providers, err
}

func (s *CatalogService) UpdateProvider(code string, input ProviderInput) (*model.SMSProvider, error) {
	var provider model.SMSProvider
	if err := s.db.Where("code = ?", code).First(&provider).Error; err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if currency == "" {
		currency = "USD"
	}
	status := input.Status
	if status == "" {
		status = model.StatusEnabled
	}
	updates := map[string]interface{}{
		"name":          input.Name,
		"base_url":      strings.TrimSpace(input.BaseURL),
		"currency_code": currency,
		"status":        status,
	}
	if strings.TrimSpace(input.APIKey) != "" {
		cipherText, err := util.EncryptString(s.encryptionKey, strings.TrimSpace(input.APIKey))
		if err != nil {
			return nil, err
		}
		updates["api_key_cipher"] = cipherText
	}
	if provider.Code == "62-us" {
		credential, shouldUpdate, err := s.sms62USCredential(provider, input)
		if err != nil {
			return nil, err
		}
		if shouldUpdate {
			cipherText, err := util.EncryptString(s.encryptionKey, credential)
			if err != nil {
				return nil, err
			}
			updates["metadata_token_cipher"] = cipherText
		}
	} else {
		credential := providerLoginCredential(input)
		if credential != "" {
			cipherText, err := util.EncryptString(s.encryptionKey, credential)
			if err != nil {
				return nil, err
			}
			updates["metadata_token_cipher"] = cipherText
		}
	}
	if err := s.db.Model(&provider).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&provider, provider.ID).Error; err != nil {
		return nil, err
	}
	if err := s.configureRuntime(provider); err != nil {
		return nil, err
	}
	s.decorateProvider(&provider)
	return &provider, nil
}

func (s *CatalogService) ConfigureRuntimeProviders() error {
	var providers []model.SMSProvider
	if err := s.db.Find(&providers).Error; err != nil {
		return err
	}
	for _, provider := range providers {
		if err := s.configureRuntime(provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *CatalogService) configureRuntime(provider model.SMSProvider) error {
	apiKey := ""
	metadataToken := ""
	if provider.APIKeyCipher != "" {
		plain, err := util.DecryptString(s.encryptionKey, provider.APIKeyCipher)
		if err != nil {
			return err
		}
		apiKey = plain
	}
	if provider.MetadataTokenCipher != "" {
		plain, err := util.DecryptString(s.encryptionKey, provider.MetadataTokenCipher)
		if err != nil {
			return err
		}
		metadataToken = normalizeLoginCredential(plain)
	}
	if s.registry == nil {
		return nil
	}
	return s.registry.ConfigureAdvanced(provider.Code, apiKey, provider.BaseURL, metadataToken)
}

func (s *CatalogService) decorateProvider(provider *model.SMSProvider) {
	provider.APIKeySet = provider.APIKeyCipher != ""
	provider.MetadataTokenSet = provider.MetadataTokenCipher != ""
	provider.LoginCredentialSet = provider.MetadataTokenSet
	provider.RequiresLoginCredential = provider.Code == "68sms" || providerCapability(provider.Capabilities, "login_credential")
	if provider.Code == "62-us" {
		provider.AuthMode = s.providerAuthMode(*provider)
	}
	if s.registry != nil {
		if smsProvider, err := s.registry.Get(provider.Code); err == nil {
			if longLived, ok := smsProvider.(sms.LongLivedProvider); ok {
				kind := longLived.ProviderKind()
				provider.ProviderKind = kind.Kind
				provider.ManualCheck = kind.ManualCheck
			}
		}
	}
}

func (s *CatalogService) providerAuthMode(provider model.SMSProvider) string {
	if provider.Code != "62-us" {
		return ""
	}
	plain := ""
	if provider.MetadataTokenCipher != "" {
		decrypted, err := util.DecryptString(s.encryptionKey, provider.MetadataTokenCipher)
		if err == nil {
			plain = decrypted
		}
	}
	credential := parseSMS62USCredential(plain)
	if credential.AuthMode != "" {
		return credential.AuthMode
	}
	if provider.APIKeyCipher != "" {
		return "openapi_token"
	}
	return "account_password"
}

type sms62USCredential struct {
	AuthMode string `json:"authMode"`
	Account  string `json:"account,omitempty"`
	Password string `json:"password,omitempty"`
}

func (s *CatalogService) sms62USCredential(provider model.SMSProvider, input ProviderInput) (string, bool, error) {
	mode := strings.TrimSpace(input.AuthMode)
	if mode == "" {
		mode = "account_password"
	}
	existing := sms62USCredential{}
	if provider.MetadataTokenCipher != "" {
		plain, err := util.DecryptString(s.encryptionKey, provider.MetadataTokenCipher)
		if err != nil {
			return "", false, err
		}
		existing = parseSMS62USCredential(plain)
	}
	if mode != "openapi_token" {
		mode = "account_password"
	}
	existing.AuthMode = mode
	if strings.TrimSpace(input.Account) != "" {
		existing.Account = strings.TrimSpace(input.Account)
	}
	if strings.TrimSpace(input.Password) != "" {
		existing.Password = strings.TrimSpace(input.Password)
	}
	if credential := providerLoginCredential(input); credential != "" && input.Account == "" && input.Password == "" {
		parsed := parseSMS62USCredential(credential)
		if parsed.AuthMode != "" {
			existing = parsed
		} else {
			existing.Account = credential
		}
	}
	raw, err := json.Marshal(existing)
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}

func parseSMS62USCredential(value string) sms62USCredential {
	value = strings.TrimSpace(value)
	if value == "" {
		return sms62USCredential{}
	}
	var credential sms62USCredential
	if err := json.Unmarshal([]byte(value), &credential); err == nil {
		credential.AuthMode = normalizeSMS62USAuthMode(credential.AuthMode)
		credential.Account = strings.TrimSpace(credential.Account)
		credential.Password = strings.TrimSpace(credential.Password)
		return credential
	}
	if strings.Contains(value, ":") {
		parts := strings.SplitN(value, ":", 2)
		return sms62USCredential{AuthMode: "account_password", Account: strings.TrimSpace(parts[0]), Password: strings.TrimSpace(parts[1])}
	}
	return sms62USCredential{AuthMode: "account_password", Account: value}
}

func normalizeSMS62USAuthMode(value string) string {
	if strings.TrimSpace(value) == "openapi_token" {
		return "openapi_token"
	}
	return "account_password"
}

func providerCapability(raw []byte, key string) bool {
	if len(raw) == 0 {
		return false
	}
	values := map[string]bool{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return false
	}
	return values[key]
}

func normalizeLoginCredential(value string) string {
	return strings.TrimSpace(value)
}

func providerLoginCredential(input ProviderInput) string {
	return normalizeLoginCredential(firstNonEmpty(input.LoginCredential, input.MetadataToken))
}

func (s *CatalogService) ListServiceConfigs() ([]model.ServiceConfig, error) {
	var configs []model.ServiceConfig
	err := s.db.Order("id desc").Find(&configs).Error
	return configs, err
}

func (s *CatalogService) CreateServiceConfig(input ServiceConfigInput) (*model.ServiceConfig, error) {
	metadata, err := normalizeServiceConfigMetadata(input.ProviderCode, input.Metadata)
	if err != nil {
		return nil, err
	}
	input.Metadata = metadata
	if input.TimeoutSeconds < 0 {
		input.TimeoutSeconds = 0
	}
	longLived := s.isLongLivedProvider(input.ProviderCode)
	if input.TimeoutSeconds == 0 && !longLived {
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
		Metadata:          input.Metadata,
		Status:            input.Status,
	}
	if err := s.db.Create(&config).Error; err != nil {
		return nil, err
	}
	if longLived && input.TimeoutSeconds == 0 {
		if err := s.db.Model(&config).Update("timeout_seconds", 0).Error; err != nil {
			return nil, err
		}
		config.TimeoutSeconds = 0
	}
	return &config, nil
}

func (s *CatalogService) UpdateServiceConfig(id uint, input ServiceConfigInput) (*model.ServiceConfig, error) {
	var config model.ServiceConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return nil, err
	}
	metadata, err := normalizeServiceConfigMetadata(input.ProviderCode, input.Metadata)
	if err != nil {
		return nil, err
	}
	input.Metadata = metadata
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
		"metadata":            input.Metadata,
		"status":              input.Status,
	}
	if updates["timeout_seconds"].(int) < 0 {
		updates["timeout_seconds"] = 0
	}
	if updates["timeout_seconds"].(int) == 0 && !s.isLongLivedProvider(input.ProviderCode) {
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

func normalizeServiceConfigMetadata(providerCode string, metadata datatypes.JSON) (datatypes.JSON, error) {
	if providerCode != "68sms" {
		return metadata, nil
	}
	values := map[string]interface{}{}
	if len(metadata) > 0 && string(metadata) != "null" {
		if err := json.Unmarshal(metadata, &values); err != nil {
			return nil, errors.New("invalid service config metadata")
		}
	}
	simType := "1"
	if value, ok := values["simType"]; ok {
		switch typed := value.(type) {
		case string:
			simType = strings.TrimSpace(typed)
		case float64:
			switch typed {
			case 1:
				simType = "1"
			case 2:
				simType = "2"
			default:
				simType = ""
			}
		default:
			simType = ""
		}
	}
	if simType != "1" && simType != "2" {
		return nil, errors.New("simType must be 1 or 2")
	}
	values["simType"] = simType
	raw, err := json.Marshal(values)
	return datatypes.JSON(raw), err
}

func (s *CatalogService) isLongLivedProvider(providerCode string) bool {
	if s.registry == nil {
		return providerCode == "68sms"
	}
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return providerCode == "68sms"
	}
	_, ok := provider.(sms.LongLivedProvider)
	return ok
}

func (s *CatalogService) DeleteServiceConfig(id uint) error {
	var count int64
	if err := s.db.Model(&model.CardBatch{}).Where("service_config_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("SERVICE_CONFIG_HAS_CARD_BATCHES: current config has card batches, please delete the card batches before deleting this config")
	}
	if err := s.dropReceiveOrderServiceConfigConstraint(); err != nil {
		return err
	}

	return s.db.Delete(&model.ServiceConfig{}, id).Error
}

func (s *CatalogService) dropReceiveOrderServiceConfigConstraint() error {
	return s.db.Exec(`
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
