package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sms-middle-platform/backend/internal/adapter/sms"
	"sms-middle-platform/backend/internal/model"

	"gorm.io/gorm"
)

type ProviderMetadataService struct {
	db       *gorm.DB
	registry *sms.Registry
}

func NewProviderMetadataService(db *gorm.DB, registry *sms.Registry) *ProviderMetadataService {
	return &ProviderMetadataService{db: db, registry: registry}
}

func (s *ProviderMetadataService) Countries(ctx context.Context, providerCode string) ([]model.ProviderCountry, error) {
	providerCode = strings.TrimSpace(providerCode)
	var countries []model.ProviderCountry
	if err := s.db.Where("provider_code = ? AND status = ?", providerCode, model.StatusEnabled).Order("name asc").Find(&countries).Error; err != nil {
		return nil, err
	}
	if len(countries) > 0 {
		return countries, nil
	}
	if err := s.SyncProviderCountries(ctx, providerCode); err != nil {
		return nil, err
	}
	return countries, s.db.Where("provider_code = ? AND status = ?", providerCode, model.StatusEnabled).Order("name asc").Find(&countries).Error
}

func (s *ProviderMetadataService) Services(ctx context.Context, providerCode, countryID string) ([]model.ProviderService, error) {
	providerCode = strings.TrimSpace(providerCode)
	countryID = strings.TrimSpace(countryID)
	var services []model.ProviderService
	query := s.db.Where("provider_code = ? AND status = ?", providerCode, model.StatusEnabled)
	if countryID != "" {
		query = query.Where("provider_country_id = ?", countryID)
	}
	if err := query.Order("name asc").Find(&services).Error; err != nil {
		return nil, err
	}
	if len(services) > 0 {
		return services, nil
	}
	if err := s.SyncProviderCountry(ctx, providerCode, countryID); err != nil {
		return nil, err
	}
	query = s.db.Where("provider_code = ? AND status = ?", providerCode, model.StatusEnabled)
	if countryID != "" {
		query = query.Where("provider_country_id = ?", countryID)
	}
	return services, query.Order("name asc").Find(&services).Error
}

func (s *ProviderMetadataService) Price(ctx context.Context, providerCode string, input sms.ProviderPriceInput) (*sms.ProviderPrice, error) {
	provider, err := s.metadataProvider(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.GetPrice(ctx, input)
}

func (s *ProviderMetadataService) Stock(ctx context.Context, providerCode string, input sms.ProviderStockInput) (*sms.ProviderStock, error) {
	provider, err := s.metadataProvider(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.GetStock(ctx, input)
}

func (s *ProviderMetadataService) Quote(ctx context.Context, providerCode string, input sms.ProviderPriceInput) (*sms.ProviderQuote, error) {
	price, err := s.Price(ctx, providerCode, input)
	if err != nil {
		return nil, err
	}
	quote := &sms.ProviderQuote{Price: price}
	stock, err := s.Stock(ctx, providerCode, sms.ProviderStockInput(input))
	if err == nil {
		quote.Stock = stock
	}
	return quote, nil
}

func (s *ProviderMetadataService) Balance(ctx context.Context, providerCode string) (*sms.ProviderBalance, error) {
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return nil, err
	}
	balance, err := provider.GetBalance(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.db.Model(&model.SMSProvider{}).
		Where("code = ?", providerCode).
		Updates(map[string]interface{}{
			"last_balance":            balance.Balance,
			"last_balance_checked_at": now,
		}).Error; err != nil {
		return nil, err
	}
	balance.CheckedAt = &now
	return balance, nil
}

func (s *ProviderMetadataService) SyncAll(ctx context.Context) error {
	var providers []model.SMSProvider
	if err := s.db.Where("status = ?", model.StatusEnabled).Order("id asc").Find(&providers).Error; err != nil {
		return err
	}
	var errs []error
	for _, provider := range providers {
		if err := s.SyncProvider(ctx, provider.Code); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", provider.Code, err))
		}
	}
	return errors.Join(errs...)
}

func (s *ProviderMetadataService) SyncProvider(ctx context.Context, providerCode string) error {
	metadataProvider, err := s.metadataProvider(providerCode)
	if err != nil {
		return err
	}
	if catalogProvider, ok := metadataProvider.(sms.CatalogProvider); ok {
		catalog, err := catalogProvider.GetCatalog(ctx)
		if err != nil {
			return err
		}
		return s.saveCatalog(providerCode, catalog)
	}
	countries, err := metadataProvider.GetCountries(ctx)
	if err != nil {
		return err
	}
	if err := s.saveCountries(providerCode, countries); err != nil {
		return err
	}
	for _, country := range countries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		countryID := providerCountryID(country)
		services, err := metadataProvider.GetServices(ctx, countryID)
		if err != nil {
			continue
		}
		if err := s.saveServices(providerCode, countryID, country.Name, services); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProviderMetadataService) SyncProviderCountries(ctx context.Context, providerCode string) error {
	metadataProvider, err := s.metadataProvider(providerCode)
	if err != nil {
		return err
	}
	if catalogProvider, ok := metadataProvider.(sms.CatalogProvider); ok {
		catalog, err := catalogProvider.GetCatalog(ctx)
		if err != nil {
			return err
		}
		return s.saveCatalog(providerCode, catalog)
	}
	countries, err := metadataProvider.GetCountries(ctx)
	if err != nil {
		return err
	}
	return s.saveCountries(providerCode, countries)
}

func (s *ProviderMetadataService) SyncProviderCountry(ctx context.Context, providerCode, countryID string) error {
	metadataProvider, err := s.metadataProvider(providerCode)
	if err != nil {
		return err
	}
	if countryID == "" {
		return s.SyncProvider(ctx, providerCode)
	}
	services, err := metadataProvider.GetServices(ctx, countryID)
	if err != nil {
		return err
	}
	var country model.ProviderCountry
	_ = s.db.Where("provider_code = ? AND provider_country_id = ?", providerCode, countryID).First(&country).Error
	return s.saveServices(providerCode, countryID, country.Name, services)
}

func (s *ProviderMetadataService) metadataProvider(providerCode string) (sms.MetadataProvider, error) {
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return nil, err
	}
	metadataProvider, ok := provider.(sms.MetadataProvider)
	if !ok {
		return nil, errors.New("provider does not support metadata queries")
	}
	return metadataProvider, nil
}

func (s *ProviderMetadataService) saveCatalog(providerCode string, catalog *sms.ProviderCatalog) error {
	if catalog == nil {
		return nil
	}
	if err := s.saveCountries(providerCode, catalog.Countries); err != nil {
		return err
	}
	countryNames := make(map[string]string, len(catalog.Countries))
	for _, country := range catalog.Countries {
		countryNames[providerCountryID(country)] = country.Name
	}
	servicesByCountry := map[string][]sms.ProviderService{}
	for _, service := range catalog.Services {
		countryID := providerServiceCountryID(service)
		servicesByCountry[countryID] = append(servicesByCountry[countryID], service)
	}
	for countryID, services := range servicesByCountry {
		if err := s.saveServices(providerCode, countryID, countryNames[countryID], services); err != nil {
			return err
		}
	}
	return nil
}

func (s *ProviderMetadataService) saveCountries(providerCode string, countries []sms.ProviderCountry) error {
	now := time.Now()
	activeIDs := make([]string, 0, len(countries))
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, country := range countries {
			countryID := providerCountryID(country)
			if countryID == "" {
				continue
			}
			activeIDs = append(activeIDs, countryID)
			record := model.ProviderCountry{
				ProviderCode:      providerCode,
				ProviderCountryID: countryID,
				Name:              cleanName(country.Name, countryID),
				ShortName:         firstNonEmpty(country.ShortName, country.Code, countryID),
				Region:            country.Region,
				DialCode:          country.DialCode,
				Status:            model.StatusEnabled,
				SyncedAt:          &now,
			}
			var existing model.ProviderCountry
			err := tx.Where("provider_code = ? AND provider_country_id = ?", providerCode, countryID).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"name":       record.Name,
				"short_name": record.ShortName,
				"region":     record.Region,
				"dial_code":  record.DialCode,
				"status":     model.StatusEnabled,
				"synced_at":  &now,
			}).Error; err != nil {
				return err
			}
		}
		if len(activeIDs) > 0 {
			return tx.Model(&model.ProviderCountry{}).
				Where("provider_code = ? AND provider_country_id NOT IN ?", providerCode, activeIDs).
				Update("status", model.StatusDisabled).Error
		}
		return nil
	})
}

func (s *ProviderMetadataService) saveServices(providerCode, countryID, countryName string, services []sms.ProviderService) error {
	now := time.Now()
	activeIDs := make([]string, 0, len(services))
	if countryName == "" {
		for _, service := range services {
			if service.CountryName != "" {
				countryName = service.CountryName
				break
			}
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, service := range services {
			serviceCountryID := firstNonEmpty(providerServiceCountryID(service), countryID)
			if serviceCountryID == "" {
				continue
			}
			serviceID := providerServiceID(service)
			if serviceID == "" {
				continue
			}
			activeIDs = append(activeIDs, serviceID)
			record := model.ProviderService{
				ProviderCode:      providerCode,
				ProviderCountryID: serviceCountryID,
				ProviderServiceID: serviceID,
				Name:              cleanName(service.Name, serviceID),
				CountryName:       firstNonEmpty(service.CountryName, countryName),
				Status:            model.StatusEnabled,
				SyncedAt:          &now,
			}
			var existing model.ProviderService
			err := tx.Where("provider_code = ? AND provider_country_id = ? AND provider_service_id = ?", providerCode, serviceCountryID, serviceID).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			updates := map[string]interface{}{
				"name":         record.Name,
				"country_name": record.CountryName,
				"status":       model.StatusEnabled,
				"synced_at":    &now,
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
		}
		if countryID != "" && len(activeIDs) > 0 {
			if err := tx.Model(&model.ProviderService{}).
				Where("provider_code = ? AND provider_country_id = ? AND provider_service_id NOT IN ?", providerCode, countryID, activeIDs).
				Update("status", model.StatusDisabled).Error; err != nil {
				return err
			}
		}
		if countryID != "" {
			return tx.Model(&model.ProviderCountry{}).
				Where("provider_code = ? AND provider_country_id = ?", providerCode, countryID).
				Update("services_synced_at", &now).Error
		}
		return nil
	})
}

func cleanName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func providerCountryID(country sms.ProviderCountry) string {
	if strings.TrimSpace(country.Code) != "" {
		return strings.TrimSpace(country.Code)
	}
	if country.ID != 0 {
		return strconv.Itoa(country.ID)
	}
	return strings.TrimSpace(country.ShortName)
}

func providerServiceID(service sms.ProviderService) string {
	if strings.TrimSpace(service.Code) != "" {
		return strings.TrimSpace(service.Code)
	}
	if service.ID != 0 {
		return strconv.Itoa(service.ID)
	}
	return ""
}

func providerServiceCountryID(service sms.ProviderService) string {
	if strings.TrimSpace(service.CountryCode) != "" {
		return strings.TrimSpace(service.CountryCode)
	}
	if service.CountryID != 0 {
		return strconv.Itoa(service.CountryID)
	}
	return ""
}
