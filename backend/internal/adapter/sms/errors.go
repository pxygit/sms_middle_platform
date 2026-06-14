package sms

import "fmt"

type ProviderError struct {
	Code    string
	Message string
}

func (e *ProviderError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewProviderError(code, message string) error {
	return &ProviderError{Code: code, Message: message}
}

const (
	ErrOutOfStock       = "OUT_OF_STOCK"
	ErrPriceNotFound    = "PRICE_NOT_FOUND"
	ErrBalance          = "BALANCE_ERROR"
	ErrOrderNotFound    = "ORDER_NOT_FOUND"
	ErrCannotCancel     = "CANNOT_CANCEL"
	ErrProviderRejected = "PROVIDER_REJECTED"
)
