package service

import (
	"context"
	"errors"

	"sms-middle-platform/backend/internal/adapter/sms"
)

type ProviderMetadataService struct {
	registry *sms.Registry
}

func NewProviderMetadataService(registry *sms.Registry) *ProviderMetadataService {
	return &ProviderMetadataService{registry: registry}
}

func (s *ProviderMetadataService) Countries(ctx context.Context, providerCode string) ([]sms.ProviderCountry, error) {
	provider, err := s.metadataProvider(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.GetCountries(ctx)
}

func (s *ProviderMetadataService) Services(ctx context.Context, providerCode, countryID string) ([]sms.ProviderService, error) {
	provider, err := s.metadataProvider(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.GetServices(ctx, countryID)
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

func (s *ProviderMetadataService) Balance(ctx context.Context, providerCode string) (*sms.ProviderBalance, error) {
	provider, err := s.registry.Get(providerCode)
	if err != nil {
		return nil, err
	}
	return provider.GetBalance(ctx)
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
