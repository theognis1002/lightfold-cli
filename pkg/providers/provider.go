package providers

import (
	"context"
	"fmt"
	"time"
)

// Provider defines the interface for cloud infrastructure providers
type Provider interface {
	// Name returns the provider name (e.g., "digitalocean", "hetzner")
	Name() string

	// DisplayName returns the human-readable provider name (e.g., "DigitalOcean", "Hetzner Cloud")
	DisplayName() string

	// SupportsProvisioning returns true if this provider can auto-provision new servers
	SupportsProvisioning() bool

	// SupportsBYOS returns true if this provider supports bring-your-own-server deployments
	SupportsBYOS() bool

	// SupportsSSH returns true if this provider uses SSH-based deployments
	// Returns false for container platforms like Fly.io that use API-based deployments
	SupportsSSH() bool

	// ValidateCredentials validates the provider's API credentials
	ValidateCredentials(ctx context.Context) error

	// GetRegions returns available regions for server deployment
	GetRegions(ctx context.Context) ([]Region, error)

	// GetSizes returns available server sizes for a given region
	GetSizes(ctx context.Context, region string) ([]Size, error)

	// GetImages returns available OS images
	GetImages(ctx context.Context) ([]Image, error)

	// Provision creates a new server with the given configuration
	Provision(ctx context.Context, config ProvisionConfig) (*Server, error)

	// GetServer retrieves server information by ID
	GetServer(ctx context.Context, serverID string) (*Server, error)

	// Destroy removes a server by ID
	Destroy(ctx context.Context, serverID string) error

	// WaitForActive polls until the server is in active state
	WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*Server, error)

	// UploadSSHKey uploads an SSH public key to the provider
	UploadSSHKey(ctx context.Context, name, publicKey string) (*SSHKey, error)
}

// Region represents a geographical region for server deployment
type Region struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

// Size represents a server size/instance type
type Size struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Memory       int     `json:"memory"` // MB
	VCPUs        int     `json:"vcpus"`
	Disk         int     `json:"disk"`          // GB
	PriceMonthly float64 `json:"price_monthly"` // USD
	PriceHourly  float64 `json:"price_hourly"`  // USD
}

// Image represents an OS image
type Image struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Distribution string `json:"distribution"`
	Version      string `json:"version"`
}

// Server represents a provisioned server
type Server struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	PublicIPv4  string            `json:"public_ipv4"`
	PrivateIPv4 string            `json:"private_ipv4"`
	Region      string            `json:"region"`
	Size        string            `json:"size"`
	Image       string            `json:"image"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
}

// SSHKey represents an SSH key for server access
type SSHKey struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
}

// ProvisionConfig contains the configuration for provisioning a new server
type ProvisionConfig struct {
	Name              string            `json:"name"`
	Region            string            `json:"region"`
	Size              string            `json:"size"`
	Image             string            `json:"image"`
	SSHKeys           []string          `json:"ssh_keys"`  // SSH key IDs
	UserData          string            `json:"user_data"` // Cloud-init script
	Tags              []string          `json:"tags"`
	Metadata          map[string]string `json:"metadata"`
	BackupsEnabled    bool              `json:"backups_enabled"`
	MonitoringEnabled bool              `json:"monitoring_enabled"`
}

// ProvisionResult contains the result of a provisioning operation
type ProvisionResult struct {
	Server   *Server  `json:"server"`
	SSHKey   *SSHKey  `json:"ssh_key,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ProviderError represents an error from a cloud provider
type ProviderError struct {
	Provider string                 `json:"provider"`
	Code     string                 `json:"code"`
	Message  string                 `json:"message"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

func (e *ProviderError) Error() string {
	if len(e.Details) == 0 {
		return e.Message
	}

	msg := e.Message

	if errDetail, ok := e.Details["error"].(string); ok && errDetail != "" {
		msg = fmt.Sprintf("%s: %s", msg, errDetail)
	}

	if apiError, ok := e.Details["api_error"].(string); ok && apiError != "" {
		msg = fmt.Sprintf("%s (API: %s)", msg, apiError)
	}

	if response, ok := e.Details["response"].(string); ok && response != "" && len(response) < 500 {
		msg = fmt.Sprintf("%s\nAPI Response: %s", msg, response)
	}

	if statusCode, ok := e.Details["status_code"].(int); ok && statusCode != 0 {
		msg = fmt.Sprintf("%s (HTTP %d)", msg, statusCode)
	}

	return msg
}
