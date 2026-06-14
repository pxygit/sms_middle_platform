package sms

import "fmt"

type Registry struct {
	providers map[string]SMSProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]SMSProvider{}}
}

func (r *Registry) Register(provider SMSProvider) {
	r.providers[provider.Name()] = provider
}

func (r *Registry) Get(name string) (SMSProvider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("sms provider %s is not registered", name)
	}
	return provider, nil
}
