package model

import (
	"time"

	"gorm.io/datatypes"
)

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	StatusVoided   = "voided"

	OrderCreated         = "created"
	OrderActive          = "active"
	OrderSMSReceived     = "sms_received"
	OrderCancelRequested = "cancel_requested"
	OrderCancelled       = "cancelled"
	OrderExpired         = "expired"
	OrderFailed          = "failed"
)

type Admin struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	Role         string     `gorm:"size:32;not null;default:admin" json:"role"`
	Status       string     `gorm:"size:32;not null;default:enabled" json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (Admin) TableName() string { return "sys_admins" }

type SMSProvider struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	Code                 string         `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name                 string         `gorm:"size:128;not null" json:"name"`
	BaseURL              string         `gorm:"size:255" json:"baseUrl"`
	CurrencyCode         string         `gorm:"size:8;not null;default:USD" json:"currencyCode"`
	APIKeyCipher         string         `gorm:"type:text" json:"-"`
	Status               string         `gorm:"size:32;not null;default:enabled" json:"status"`
	Capabilities         datatypes.JSON `gorm:"type:jsonb" json:"capabilities"`
	LastBalance          string         `gorm:"size:64" json:"lastBalance"`
	LastBalanceCheckedAt *time.Time     `json:"lastBalanceCheckedAt"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
	APIKeySet            bool           `gorm:"-" json:"apiKeySet"`
}

func (SMSProvider) TableName() string { return "sys_sms_providers" }

type ProviderCountry struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ProviderCode      string     `gorm:"size:64;uniqueIndex:idx_provider_country;index;not null" json:"providerCode"`
	ProviderCountryID string     `gorm:"size:64;uniqueIndex:idx_provider_country;not null" json:"providerCountryId"`
	Name              string     `gorm:"size:128;not null" json:"name"`
	ShortName         string     `gorm:"size:64" json:"shortName"`
	Region            string     `gorm:"size:128" json:"region"`
	DialCode          string     `gorm:"size:32" json:"dialCode"`
	Status            string     `gorm:"size:32;not null;default:enabled" json:"status"`
	SyncedAt          *time.Time `json:"syncedAt"`
	ServicesSyncedAt  *time.Time `json:"servicesSyncedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (ProviderCountry) TableName() string { return "sys_provider_countries" }

// ProviderService caches provider service options for fast searchable dropdowns.
// Real-time phone stock and price metrics are fetched from provider APIs on demand.
type ProviderService struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ProviderCode      string     `gorm:"size:64;uniqueIndex:idx_provider_service;index;not null" json:"providerCode"`
	ProviderCountryID string     `gorm:"size:64;uniqueIndex:idx_provider_service;index;not null" json:"providerCountryId"`
	ProviderServiceID string     `gorm:"size:64;uniqueIndex:idx_provider_service;not null" json:"providerServiceId"`
	Name              string     `gorm:"size:160;not null" json:"name"`
	CountryName       string     `gorm:"size:128" json:"countryName"`
	Status            string     `gorm:"size:32;not null;default:enabled" json:"status"`
	SyncedAt          *time.Time `json:"syncedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (ProviderService) TableName() string { return "sys_provider_services" }

type ServiceConfig struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	ProviderCode      string    `gorm:"size:64;index;not null" json:"providerCode"`
	TargetPlatform    string    `gorm:"size:128;index;not null" json:"targetPlatform"`
	DisplayName       string    `gorm:"size:128;not null" json:"displayName"`
	CountryCode       string    `gorm:"size:16;not null" json:"countryCode"`
	CountryName       string    `gorm:"size:128" json:"countryName"`
	ProviderCountryID string    `gorm:"size:64;not null" json:"providerCountryId"`
	ProviderServiceID string    `gorm:"size:64;not null" json:"providerServiceId"`
	ProviderPoolID    string    `gorm:"size:64" json:"providerPoolId"`
	MaxPrice          float64   `gorm:"type:numeric(12,4);not null;default:0" json:"maxPrice"`
	TimeoutSeconds    int       `gorm:"not null;default:1200" json:"timeoutSeconds"`
	Status            string    `gorm:"size:32;not null;default:enabled" json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (ServiceConfig) TableName() string { return "sys_service_configs" }

type CardBatch struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `gorm:"size:128;not null" json:"name"`
	ProviderCode    string     `gorm:"size:64;index;not null" json:"providerCode"`
	ServiceConfigID uint       `gorm:"index;not null" json:"serviceConfigId"`
	Quantity        int        `gorm:"not null" json:"quantity"`
	UsesPerCode     int        `gorm:"not null" json:"usesPerCode"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	ExportPath      string     `gorm:"size:512" json:"-"`
	ExportedAt      *time.Time `json:"exportedAt"`
	CreatedBy       uint       `gorm:"index" json:"createdBy"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (CardBatch) TableName() string { return "sys_card_batches" }

type CardCode struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	CodeHash        string     `gorm:"size:128;uniqueIndex;not null" json:"-"`
	CodeMask        string     `gorm:"size:64;not null" json:"codeMask"`
	CodeCipher      string     `gorm:"type:text" json:"-"`
	ProviderCode    string     `gorm:"size:64;index;not null" json:"providerCode"`
	ServiceConfigID uint       `gorm:"index;not null" json:"serviceConfigId"`
	BatchID         uint       `gorm:"index;not null" json:"batchId"`
	TotalUses       int        `gorm:"not null" json:"totalUses"`
	RemainingUses   int        `gorm:"not null" json:"remainingUses"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	Status          string     `gorm:"size:32;index;not null;default:enabled" json:"status"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`

	ServiceConfig ServiceConfig `gorm:"foreignKey:ServiceConfigID" json:"serviceConfig"`
}

func (CardCode) TableName() string { return "sys_card_codes" }

type ReceiveOrder struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	OrderNo             string         `gorm:"size:64;uniqueIndex;not null" json:"orderNo"`
	CardCodeID          uint           `gorm:"index;not null" json:"cardCodeId"`
	ProviderCode        string         `gorm:"size:64;index;not null" json:"providerCode"`
	ServiceConfigID     uint           `gorm:"index;not null" json:"serviceConfigId"`
	SupplierOrderID     string         `gorm:"size:128;index" json:"supplierOrderId"`
	SupplierToken       string         `gorm:"size:255" json:"supplierToken"`
	PhoneNumber         string         `gorm:"size:64" json:"phoneNumber"`
	PhoneCountryCode    string         `gorm:"size:16" json:"phoneCountryCode"`
	PhoneNationalNumber string         `gorm:"size:64" json:"phoneNationalNumber"`
	VerificationCode    string         `gorm:"size:128" json:"verificationCode"`
	SMSContent          string         `gorm:"type:text" json:"smsContent"`
	Cost                float64        `gorm:"type:numeric(12,4);not null;default:0" json:"cost"`
	MaxPrice            float64        `gorm:"type:numeric(12,4);not null;default:0" json:"maxPrice"`
	Status              string         `gorm:"size:32;index;not null;default:created" json:"status"`
	SupplierStatus      string         `gorm:"size:64" json:"supplierStatus"`
	RawResponse         datatypes.JSON `gorm:"type:jsonb" json:"rawResponse"`
	FailureReason       string         `gorm:"type:text" json:"failureReason"`
	StartedAt           *time.Time     `json:"startedAt"`
	ReceivedAt          *time.Time     `json:"receivedAt"`
	CancelledAt         *time.Time     `json:"cancelledAt"`
	ExpiredAt           *time.Time     `json:"expiredAt"`
	LastPolledAt        *time.Time     `json:"lastPolledAt"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`

	ServiceConfig ServiceConfig `gorm:"foreignKey:ServiceConfigID" json:"serviceConfig"`
}

func (ReceiveOrder) TableName() string { return "sys_receive_orders" }

type SupplierRequestLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ProviderCode string    `gorm:"size:64;index;not null" json:"providerCode"`
	Action       string    `gorm:"size:64;index;not null" json:"action"`
	RequestID    string    `gorm:"size:128;index" json:"requestId"`
	HTTPStatus   int       `json:"httpStatus"`
	Success      bool      `gorm:"index" json:"success"`
	ErrorCode    string    `gorm:"size:128" json:"errorCode"`
	ErrorMessage string    `gorm:"type:text" json:"errorMessage"`
	LatencyMS    int64     `json:"latencyMs"`
	RequestBody  string    `gorm:"type:text" json:"requestBody"`
	ResponseBody string    `gorm:"type:text" json:"responseBody"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (SupplierRequestLog) TableName() string { return "sys_supplier_request_logs" }

type AuditLog struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ActorType    string         `gorm:"size:32;index;not null" json:"actorType"`
	ActorID      uint           `gorm:"index" json:"actorId"`
	Action       string         `gorm:"size:128;index;not null" json:"action"`
	ResourceType string         `gorm:"size:64;index" json:"resourceType"`
	ResourceID   string         `gorm:"size:64;index" json:"resourceId"`
	IP           string         `gorm:"size:64" json:"ip"`
	UserAgent    string         `gorm:"size:255" json:"userAgent"`
	Metadata     datatypes.JSON `gorm:"type:jsonb" json:"metadata"`
	CreatedAt    time.Time      `json:"createdAt"`
}

func (AuditLog) TableName() string { return "sys_audit_logs" }

type SiteVisit struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"size:128;index;not null" json:"path"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"userAgent"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

func (SiteVisit) TableName() string { return "sys_site_visits" }
