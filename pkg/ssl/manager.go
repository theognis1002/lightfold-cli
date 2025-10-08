package ssl

import (
	"fmt"
	"sync"
)

// SSLManager defines the interface for SSL certificate management
type SSLManager interface {
	// IsAvailable checks if the SSL manager is available on the system
	IsAvailable() (bool, error)

	// IssueCertificate issues a new SSL certificate for the given domain
	IssueCertificate(domain string, email string) error

	// RenewCertificate renews an existing SSL certificate for the given domain
	RenewCertificate(domain string) error

	// EnableAutoRenewal sets up automatic certificate renewal
	EnableAutoRenewal() error

	// GetCertificatePath returns the path to the certificate files
	GetCertificatePath(domain string) (certPath string, keyPath string, error error)

	// Name returns the name of the SSL manager
	Name() string
}

// SSLManagerFactory is a function that creates a new SSLManager instance
type SSLManagerFactory func() SSLManager

var (
	registry = make(map[string]SSLManagerFactory)
	mu       sync.RWMutex
)

// Register registers a new SSL manager factory with the given name
func Register(name string, factory SSLManagerFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Get retrieves an SSL manager by name
func Get(name string) (SSLManager, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("SSL manager '%s' not found", name)
	}

	return factory(), nil
}

// List returns all registered SSL manager names
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// GetDefault returns the default SSL manager (certbot)
func GetDefault() (SSLManager, error) {
	return Get("certbot")
}

// GetManager is an alias for Get to match the naming convention
func GetManager(name string) (SSLManager, error) {
	return Get(name)
}
