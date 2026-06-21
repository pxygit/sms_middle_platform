package sms

import (
	"context"
	"encoding/json"
	"time"
)

type ProviderBalance struct {
	Balance   string     `json:"balance"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

type RequestNumberInput struct {
	CountryID string
	ServiceID string
	PoolID    string
	MaxPrice  float64
	Metadata  json.RawMessage
}

type RequestNumberResult struct {
	SupplierOrderID     string
	SupplierToken       string
	PhoneNumber         string
	PhoneCountryCode    string
	PhoneNationalNumber string
	Country             string
	Service             string
	Cost                float64
	ExpiresIn           int
	Expiration          int64
	Raw                 json.RawMessage
}

type CheckSMSInput struct {
	SupplierOrderID string
}

type ManualSMSInput struct {
	SupplierOrderID string
	SupplierToken   string
}

type SMSResult struct {
	Status           string
	SupplierStatus   string
	VerificationCode string
	SMSContent       string
	Expiration       int64
	TimeLeft         int
	Raw              json.RawMessage
}

type CancelNumberInput struct {
	SupplierOrderID string
}

type CancelResult struct {
	Success bool
	Message string
	Raw     json.RawMessage
}

type ProviderCountry struct {
	ID        int    `json:"id"`
	Code      string `json:"code,omitempty"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	Region    string `json:"region"`
	DialCode  string `json:"dialCode,omitempty"`
}

type ProviderService struct {
	ID          int    `json:"id"`
	Code        string `json:"code,omitempty"`
	Name        string `json:"name"`
	Favourite   int    `json:"favourite"`
	CountryID   int    `json:"countryId,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	CountryName string `json:"countryName,omitempty"`
	Price       string `json:"price,omitempty"`
	Stock       int    `json:"stock,omitempty"`
}

type ProviderPriceInput struct {
	CountryID string
	ServiceID string
	PoolID    string
}

type ProviderPrice struct {
	Pool        int             `json:"pool"`
	LowPrice    string          `json:"lowPrice"`
	HighPrice   string          `json:"highPrice"`
	Price       string          `json:"price"`
	SuccessRate float64         `json:"successRate"`
	Raw         json.RawMessage `json:"-"`
}

type ProviderStockInput struct {
	CountryID string
	ServiceID string
	PoolID    string
}

type ProviderStock struct {
	Amount int             `json:"amount"`
	Raw    json.RawMessage `json:"-"`
}

type ProviderQuote struct {
	Price *ProviderPrice `json:"price,omitempty"`
	Stock *ProviderStock `json:"stock,omitempty"`
}

type ValidityOptionsInput struct {
	CountryID string
	ServiceID string
	PoolID    string
}

type ProviderValidityOption struct {
	Value   string          `json:"value"`
	Label   string          `json:"label"`
	MinDays int             `json:"minDays"`
	MaxDays int             `json:"maxDays"`
	Stock   int             `json:"stock"`
	Raw     json.RawMessage `json:"-"`
}

type ProviderKind struct {
	Kind               string `json:"kind"`
	ManualCheck        bool   `json:"manualCheck"`
	MessageURLTemplate string `json:"messageUrlTemplate,omitempty"`
}

type ProviderCatalog struct {
	Countries []ProviderCountry
	Services  []ProviderService
}

type SMSProvider interface {
	Name() string
	GetBalance(ctx context.Context) (*ProviderBalance, error)
	RequestNumber(ctx context.Context, input RequestNumberInput) (*RequestNumberResult, error)
	CheckSMS(ctx context.Context, input CheckSMSInput) (*SMSResult, error)
	CancelNumber(ctx context.Context, input CancelNumberInput) (*CancelResult, error)
}

type MetadataProvider interface {
	GetCountries(ctx context.Context) ([]ProviderCountry, error)
	GetServices(ctx context.Context, countryID string) ([]ProviderService, error)
	GetPrice(ctx context.Context, input ProviderPriceInput) (*ProviderPrice, error)
	GetStock(ctx context.Context, input ProviderStockInput) (*ProviderStock, error)
}

type CatalogProvider interface {
	GetCatalog(ctx context.Context) (*ProviderCatalog, error)
}

type ValidityOptionsProvider interface {
	GetValidityOptions(ctx context.Context, input ValidityOptionsInput) ([]ProviderValidityOption, error)
}

type ConfigurableProvider interface {
	Configure(apiKey, baseURL string)
}

type AdvancedConfigurableProvider interface {
	ConfigureAdvanced(apiKey, baseURL, metadataToken string)
}

type LongLivedProvider interface {
	ProviderKind() ProviderKind
	GetMessageURL(token string) string
	CheckManualSMS(ctx context.Context, input ManualSMSInput) (*SMSResult, error)
}
