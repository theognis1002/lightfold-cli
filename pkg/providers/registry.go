package providers

import (
	"fmt"
	"sync"
)

// ProviderFactory creates a new provider instance with the given API token
type ProviderFactory func(token string) Provider

// Registry manages registered cloud providers
type Registry struct {
	mu        sync.RWMutex
	providers map[string]ProviderFactory
}

var globalRegistry = &Registry{
	providers: make(map[string]ProviderFactory),
}

// Register adds a provider factory to the global registry
// This should be called in init() functions of provider packages
func Register(name string, factory ProviderFactory) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.providers[name] = factory
}

// GetProvider creates a provider instance by name with the given token
func GetProvider(name, token string) (Provider, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	factory, exists := globalRegistry.providers[name]
	if !exists {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}

	return factory(token), nil
}

// ListProviders returns all registered provider names
func ListProviders() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	names := make([]string, 0, len(globalRegistry.providers))
	for name := range globalRegistry.providers {
		names = append(names, name)
	}
	return names
}

// IsRegistered checks if a provider is registered
func IsRegistered(name string) bool {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	_, exists := globalRegistry.providers[name]
	return exists
}

// ProviderInfo contains metadata about a provider
type ProviderInfo struct {
	Name                 string
	DisplayName          string
	SupportsProvisioning bool
	SupportsBYOS         bool
}

// GetProviderInfo returns metadata for a provider without creating an instance
// This creates a temporary instance with an empty token just to read metadata
func GetProviderInfo(name, token string) (*ProviderInfo, error) {
	provider, err := GetProvider(name, token)
	if err != nil {
		return nil, err
	}

	return &ProviderInfo{
		Name:                 provider.Name(),
		DisplayName:          provider.DisplayName(),
		SupportsProvisioning: provider.SupportsProvisioning(),
		SupportsBYOS:         provider.SupportsBYOS(),
	}, nil
}

// GetAllProviderInfo returns metadata for all registered providers
func GetAllProviderInfo() ([]*ProviderInfo, error) {
	names := ListProviders()
	infos := make([]*ProviderInfo, 0, len(names))

	for _, name := range names {
		// Use empty token for metadata retrieval
		info, err := GetProviderInfo(name, "")
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}
