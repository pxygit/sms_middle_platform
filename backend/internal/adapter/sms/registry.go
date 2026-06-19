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

func (r *Registry) Configure(name, apiKey, baseURL string) error {
	provider, err := r.Get(name)
	if err != nil {
		return err
	}
	if configurable, ok := provider.(AdvancedConfigurableProvider); ok {
		configurable.ConfigureAdvanced(apiKey, baseURL, "")
		return nil
	}
	configurable, ok := provider.(ConfigurableProvider)
	if !ok {
		return fmt.Errorf("sms provider %s does not support runtime configuration", name)
	}
	configurable.Configure(apiKey, baseURL)
	return nil
}

func (r *Registry) ConfigureAdvanced(name, apiKey, baseURL, metadataToken string) error {
	provider, err := r.Get(name)
	if err != nil {
		return err
	}
	if configurable, ok := provider.(AdvancedConfigurableProvider); ok {
		configurable.ConfigureAdvanced(apiKey, baseURL, metadataToken)
		return nil
	}
	if metadataToken != "" {
		return fmt.Errorf("sms provider %s does not support metadata token configuration", name)
	}
	configurable, ok := provider.(ConfigurableProvider)
	if !ok {
		return fmt.Errorf("sms provider %s does not support runtime configuration", name)
	}
	configurable.Configure(apiKey, baseURL)
	return nil
}
