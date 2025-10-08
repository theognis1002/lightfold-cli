package proxy

import (
	"fmt"
	"sync"
)

// ProxyConfig contains configuration for setting up a reverse proxy
type ProxyConfig struct {
	Domain      string
	Port        int
	AppName     string
	SSLEnabled  bool
	SSLCertPath string
	SSLKeyPath  string
}

// ProxyManager defines the interface for reverse proxy management
type ProxyManager interface {
	// Configure sets up the reverse proxy configuration
	Configure(config ProxyConfig) error

	// Reload reloads the proxy configuration
	Reload() error

	// Remove removes the proxy configuration
	Remove(appName string) error

	// GetConfigPath returns the path to the configuration file
	GetConfigPath(appName string) string

	// IsAvailable checks if the proxy is available on the system
	IsAvailable() (bool, error)

	// Name returns the name of the proxy manager
	Name() string
}

// ProxyManagerFactory is a function that creates a new ProxyManager instance
type ProxyManagerFactory func() ProxyManager

var (
	registry = make(map[string]ProxyManagerFactory)
	mu       sync.RWMutex
)

// Register registers a new proxy manager factory with the given name
func Register(name string, factory ProxyManagerFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// Get retrieves a proxy manager by name
func Get(name string) (ProxyManager, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("proxy manager '%s' not found", name)
	}

	return factory(), nil
}

// List returns all registered proxy manager names
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// GetDefault returns the default proxy manager (nginx)
func GetDefault() (ProxyManager, error) {
	return Get("nginx")
}

// GetManager is an alias for Get to match the naming convention
func GetManager(name string) (ProxyManager, error) {
	return Get(name)
}
