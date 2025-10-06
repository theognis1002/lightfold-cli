package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderConfig is a generic interface for provider-specific configurations
type ProviderConfig interface {
	GetIP() string
	GetUsername() string
	GetSSHKey() string
	IsProvisioned() bool
}

// DigitalOceanConfig contains DigitalOcean-specific deployment configuration
type DigitalOceanConfig struct {
	DropletID   string `json:"droplet_id,omitempty"` // For provisioned droplets
	IP          string `json:"ip"`
	SSHKey      string `json:"ssh_key"`
	SSHKeyName  string `json:"ssh_key_name,omitempty"`
	Username    string `json:"username"`
	Region      string `json:"region,omitempty"`
	Size        string `json:"size,omitempty"`
	Provisioned bool   `json:"provisioned,omitempty"`
}

func (d *DigitalOceanConfig) GetIP() string        { return d.IP }
func (d *DigitalOceanConfig) GetUsername() string  { return d.Username }
func (d *DigitalOceanConfig) GetSSHKey() string    { return d.SSHKey }
func (d *DigitalOceanConfig) IsProvisioned() bool  { return d.Provisioned }

// HetznerConfig contains Hetzner-specific deployment configuration
type HetznerConfig struct {
	ServerID    string `json:"server_id,omitempty"`
	IP          string `json:"ip"`
	SSHKey      string `json:"ssh_key"`
	SSHKeyName  string `json:"ssh_key_name,omitempty"`
	Username    string `json:"username"`
	Location    string `json:"location,omitempty"`
	ServerType  string `json:"server_type,omitempty"`
	Provisioned bool   `json:"provisioned,omitempty"`
}

func (h *HetznerConfig) GetIP() string        { return h.IP }
func (h *HetznerConfig) GetUsername() string  { return h.Username }
func (h *HetznerConfig) GetSSHKey() string    { return h.SSHKey }
func (h *HetznerConfig) IsProvisioned() bool  { return h.Provisioned }

// VultrConfig contains Vultr-specific deployment configuration
type VultrConfig struct {
	InstanceID  string `json:"instance_id,omitempty"` // For provisioned instances
	IP          string `json:"ip"`
	SSHKey      string `json:"ssh_key"`
	SSHKeyName  string `json:"ssh_key_name,omitempty"`
	Username    string `json:"username"`
	Region      string `json:"region,omitempty"`
	Plan        string `json:"plan,omitempty"`       // Vultr uses "plan" instead of "size"
	Provisioned bool   `json:"provisioned,omitempty"`
}

func (v *VultrConfig) GetIP() string        { return v.IP }
func (v *VultrConfig) GetUsername() string  { return v.Username }
func (v *VultrConfig) GetSSHKey() string    { return v.SSHKey }
func (v *VultrConfig) IsProvisioned() bool  { return v.Provisioned }

// S3Config contains S3-specific deployment configuration
type S3Config struct {
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

func (s *S3Config) GetIP() string        { return "" }
func (s *S3Config) GetUsername() string  { return "" }
func (s *S3Config) GetSSHKey() string    { return "" }
func (s *S3Config) IsProvisioned() bool  { return false }

// DeploymentOptions contains framework-agnostic deployment settings
type DeploymentOptions struct {
	SkipBuild     bool              `json:"skip_build,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	BuildCommand  string            `json:"build_command,omitempty"`
	RunCommand    string            `json:"run_command,omitempty"`
	BuildCommands []string          `json:"build_commands,omitempty"`
	RunCommands   []string          `json:"run_commands,omitempty"`
}

// TargetConfig contains the complete target deployment configuration
type TargetConfig struct {
	ProjectPath    string                     `json:"project_path"`
	Framework      string                     `json:"framework"`
	Provider       string                     `json:"provider"`
	Builder        string                     `json:"builder,omitempty"`
	ProviderConfig map[string]json.RawMessage `json:"provider_config"`
	Deploy         *DeploymentOptions         `json:"deploy,omitempty"`
}

// GetProviderConfig unmarshals the provider-specific config into the given type
func (t *TargetConfig) GetProviderConfig(provider string, target interface{}) error {
	if t.ProviderConfig == nil {
		return fmt.Errorf("no provider configuration found")
	}

	configJSON, exists := t.ProviderConfig[provider]
	if !exists {
		return fmt.Errorf("no configuration found for provider: %s", provider)
	}

	return json.Unmarshal(configJSON, target)
}

// SetProviderConfig marshals and stores provider-specific configuration
func (t *TargetConfig) SetProviderConfig(provider string, config interface{}) error {
	if t.ProviderConfig == nil {
		t.ProviderConfig = make(map[string]json.RawMessage)
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	t.ProviderConfig[provider] = configJSON
	return nil
}

// GetDigitalOceanConfig is a convenience method to get DigitalOcean configuration
func (t *TargetConfig) GetDigitalOceanConfig() (*DigitalOceanConfig, error) {
	var config DigitalOceanConfig
	if err := t.GetProviderConfig("digitalocean", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetHetznerConfig is a convenience method to get Hetzner configuration
func (t *TargetConfig) GetHetznerConfig() (*HetznerConfig, error) {
	var config HetznerConfig
	if err := t.GetProviderConfig("hetzner", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetS3Config is a convenience method to get S3 configuration
func (t *TargetConfig) GetS3Config() (*S3Config, error) {
	var config S3Config
	if err := t.GetProviderConfig("s3", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetVultrConfig is a convenience method to get Vultr configuration
func (t *TargetConfig) GetVultrConfig() (*VultrConfig, error) {
	var config VultrConfig
	if err := t.GetProviderConfig("vultr", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetSSHProviderConfig returns the appropriate ProviderConfig for SSH-based providers
// This eliminates the need for repetitive switch statements throughout the codebase
func (t *TargetConfig) GetSSHProviderConfig() (ProviderConfig, error) {
	switch t.Provider {
	case "digitalocean":
		return t.GetDigitalOceanConfig()
	case "hetzner":
		return t.GetHetznerConfig()
	case "vultr":
		return t.GetVultrConfig()
	case "s3":
		return nil, fmt.Errorf("S3 is not an SSH-based provider")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", t.Provider)
	}
}

// GetAnyProviderConfig returns the appropriate ProviderConfig for any provider
func (t *TargetConfig) GetAnyProviderConfig() (ProviderConfig, error) {
	switch t.Provider {
	case "digitalocean":
		return t.GetDigitalOceanConfig()
	case "hetzner":
		return t.GetHetznerConfig()
	case "vultr":
		return t.GetVultrConfig()
	case "s3":
		return t.GetS3Config()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", t.Provider)
	}
}

type Config struct {
	Targets      map[string]TargetConfig `json:"targets"`
	KeepReleases int                     `json:"keep_releases,omitempty"` // Number of releases to keep (default: 5)
}

func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(LocalConfigDir, LocalConfigFile)
	}
	return filepath.Join(homeDir, LocalConfigDir, LocalConfigFile)
}

func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, PermDirectory); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{
			Targets:      make(map[string]TargetConfig),
			KeepReleases: DefaultKeepReleases,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Targets == nil {
		config.Targets = make(map[string]TargetConfig)
	}

	if config.KeepReleases == 0 {
		config.KeepReleases = DefaultKeepReleases
	}

	return &config, nil
}

func (c *Config) SaveConfig() error {
	configPath := GetConfigPath()

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, PermDirectory); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, PermConfigFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetTarget retrieves a target configuration by name
func (c *Config) GetTarget(targetName string) (TargetConfig, bool) {
	target, exists := c.Targets[targetName]
	return target, exists
}

// SetTarget stores a target configuration by name
func (c *Config) SetTarget(targetName string, target TargetConfig) error {
	c.Targets[targetName] = target
	return nil
}

// DeleteTarget removes a target configuration and saves the config
func (c *Config) DeleteTarget(targetName string) error {
	if _, exists := c.Targets[targetName]; !exists {
		return nil
	}

	delete(c.Targets, targetName)
	return c.SaveConfig()
}

// FindTargetByPath finds a target by its project path
func (c *Config) FindTargetByPath(projectPath string) (string, TargetConfig, bool) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", TargetConfig{}, false
	}

	for name, target := range c.Targets {
		if target.ProjectPath == absPath {
			return name, target, true
		}
	}
	return "", TargetConfig{}, false
}

// TokenConfig stores API tokens for all providers
type TokenConfig struct {
	Tokens map[string]string `json:"tokens,omitempty"` // provider_name -> token
}

func GetTokensPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(LocalConfigDir, LocalTokensFile)
	}
	return filepath.Join(homeDir, LocalConfigDir, LocalTokensFile)
}

func LoadTokens() (*TokenConfig, error) {
	tokensPath := GetTokensPath()

	dir := filepath.Dir(tokensPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create tokens directory: %w", err)
	}

	if _, err := os.Stat(tokensPath); os.IsNotExist(err) {
		return &TokenConfig{
			Tokens: make(map[string]string),
		}, nil
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokens file: %w", err)
	}

	var tokens TokenConfig
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse tokens file: %w", err)
	}

	if tokens.Tokens == nil {
		tokens.Tokens = make(map[string]string)
	}

	return &tokens, nil
}

func (t *TokenConfig) SaveTokens() error {
	tokensPath := GetTokensPath()

	dir := filepath.Dir(tokensPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	if err := os.WriteFile(tokensPath, data, PermTokenFile); err != nil {
		return fmt.Errorf("failed to write tokens file: %w", err)
	}

	return nil
}

func (t *TokenConfig) SetToken(provider, token string) {
	if t.Tokens == nil {
		t.Tokens = make(map[string]string)
	}

	for {
		oldToken := token

		token = strings.TrimSpace(token)

		token = strings.TrimPrefix(token, "[")
		token = strings.TrimSuffix(token, "]")

		token = strings.TrimSpace(token)

		token = strings.Trim(token, "\"'")

		if token == oldToken {
			break
		}
	}

	t.Tokens[provider] = token
}

// GetToken retrieves an API token for a specific provider
func (t *TokenConfig) GetToken(provider string) string {
	if t.Tokens == nil {
		return ""
	}
	return t.Tokens[provider]
}

// HasToken checks if a token exists for the given provider
func (t *TokenConfig) HasToken(provider string) bool {
	return t.GetToken(provider) != ""
}

// SetDigitalOceanToken is a convenience method for setting the DigitalOcean token
func (t *TokenConfig) SetDigitalOceanToken(token string) {
	t.SetToken("digitalocean", token)
}

// GetDigitalOceanToken is a convenience method for getting the DigitalOcean token
func (t *TokenConfig) GetDigitalOceanToken() string {
	return t.GetToken("digitalocean")
}
