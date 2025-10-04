package config

import "time"

// Timeouts & Durations
const (
	// DefaultHealthCheckTimeout is the default timeout for health check requests
	DefaultHealthCheckTimeout = 30 * time.Second

	// DefaultCloudInitTimeout is the timeout for waiting for cloud-init to complete
	DefaultCloudInitTimeout = 300 * time.Second // 5 minutes

	// DefaultSSHTimeout is the default timeout for SSH connections
	DefaultSSHTimeout = 30 * time.Second

	// DefaultProvisioningTimeout is the timeout for provisioning operations
	DefaultProvisioningTimeout = 10 * time.Minute

	// DefaultDestroyTimeout is the timeout for destroy operations
	DefaultDestroyTimeout = 5 * time.Minute

	// DefaultHealthCheckRetryDelay is the delay between health check retries
	DefaultHealthCheckRetryDelay = 3 * time.Second

	// DefaultAptRetryDelay is the base delay for APT retry operations
	DefaultAptRetryDelay = 2 * time.Second
)

// Retry Counts
const (
	// DefaultAptMaxRetries is the maximum number of retries for APT operations
	DefaultAptMaxRetries = 3

	// DefaultHealthCheckMaxRetries is the maximum number of health check retries
	DefaultHealthCheckMaxRetries = 5
)

// File Permissions
const (
	// PermPrivateKey is the file permission for private SSH keys
	PermPrivateKey = 0600

	// PermPublicKey is the file permission for public SSH keys
	PermPublicKey = 0644

	// PermDirectory is the file permission for directories
	PermDirectory = 0755

	// PermEnvFile is the file permission for environment files
	PermEnvFile = 0600

	// PermConfigFile is the file permission for config files
	PermConfigFile = 0644

	// PermTokenFile is the file permission for token files (sensitive)
	PermTokenFile = 0600
)

// Path Constants - Local
const (
	// LocalConfigDir is the base directory for lightfold configuration
	LocalConfigDir = ".lightfold"

	// LocalConfigFile is the filename for the main config
	LocalConfigFile = "config.json"

	// LocalTokensFile is the filename for API tokens
	LocalTokensFile = "tokens.json"

	// LocalStateDir is the directory name for state files
	LocalStateDir = "state"

	// LocalKeysDir is the directory name for SSH keys
	LocalKeysDir = "keys"
)

// Path Constants - Remote Server
const (
	// RemoteLightfoldDir is the directory on remote servers for lightfold markers
	RemoteLightfoldDir = "/etc/lightfold"

	// RemoteCreatedMarker is the filename for the "created" marker
	RemoteCreatedMarker = "created"

	// RemoteConfiguredMarker is the filename for the "configured" marker
	RemoteConfiguredMarker = "configured"

	// RemoteAppBaseDir is the base directory for deployed applications
	RemoteAppBaseDir = "/srv"
)

// Default Values
const (
	// DefaultKeepReleases is the default number of releases to keep
	DefaultKeepReleases = 5

	// DefaultSSHPort is the default SSH port
	DefaultSSHPort = "22"

	// DefaultHealthCheckStatus is the default expected HTTP status for health checks
	DefaultHealthCheckStatus = 200

	// DefaultRebootDelayMinutes is the delay in minutes before system reboot after updates
	DefaultRebootDelayMinutes = 5
)
