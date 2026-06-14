package sms

import (
	"context"
	"encoding/json"
)

type ProviderBalance struct {
	Balance string `json:"balance"`
}

type RequestNumberInput struct {
	CountryID string
	ServiceID string
	PoolID    string
	MaxPrice  float64
}

type RequestNumberResult struct {
	SupplierOrderID string
	SupplierToken   string
	PhoneNumber     string
	Country         string
	Service         string
	Cost            float64
	ExpiresIn       int
	Expiration      int64
	Raw             json.RawMessage
}

type CheckSMSInput struct {
	SupplierOrderID string
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

type SMSProvider interface {
	Name() string
	GetBalance(ctx context.Context) (*ProviderBalance, error)
	RequestNumber(ctx context.Context, input RequestNumberInput) (*RequestNumberResult, error)
	CheckSMS(ctx context.Context, input CheckSMSInput) (*SMSResult, error)
	CancelNumber(ctx context.Context, input CancelNumberInput) (*CancelResult, error)
}
