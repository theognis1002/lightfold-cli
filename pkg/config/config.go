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

// S3Config contains S3-specific deployment configuration
type S3Config struct {
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

// GetIP returns empty string as S3 doesn't use IP addresses
func (s *S3Config) GetIP() string        { return "" }
func (s *S3Config) GetUsername() string  { return "" }
func (s *S3Config) GetSSHKey() string    { return "" }
func (s *S3Config) IsProvisioned() bool  { return false }

// DeploymentOptions contains framework-agnostic deployment settings
type DeploymentOptions struct {
	SkipBuild    bool              `json:"skip_build,omitempty"`
	EnvVars      map[string]string `json:"env_vars,omitempty"`
	BuildCommand string            `json:"build_command,omitempty"`
	RunCommand   string            `json:"run_command,omitempty"`
}

// ProjectConfig contains the complete project deployment configuration
type ProjectConfig struct {
	Framework      string                     `json:"framework"`
	Provider       string                     `json:"provider"`        // Provider name (e.g., "digitalocean", "hetzner", "s3")
	ProviderConfig map[string]json.RawMessage `json:"provider_config"` // Provider-specific config as JSON
	Deploy         *DeploymentOptions         `json:"deploy,omitempty"`
}

// GetProviderConfig unmarshals the provider-specific config into the given type
func (p *ProjectConfig) GetProviderConfig(provider string, target interface{}) error {
	if p.ProviderConfig == nil {
		return fmt.Errorf("no provider configuration found")
	}

	configJSON, exists := p.ProviderConfig[provider]
	if !exists {
		return fmt.Errorf("no configuration found for provider: %s", provider)
	}

	return json.Unmarshal(configJSON, target)
}

// SetProviderConfig marshals and stores provider-specific configuration
func (p *ProjectConfig) SetProviderConfig(provider string, config interface{}) error {
	if p.ProviderConfig == nil {
		p.ProviderConfig = make(map[string]json.RawMessage)
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal provider config: %w", err)
	}

	p.ProviderConfig[provider] = configJSON
	return nil
}

// GetDigitalOceanConfig is a convenience method to get DigitalOcean configuration
func (p *ProjectConfig) GetDigitalOceanConfig() (*DigitalOceanConfig, error) {
	var config DigitalOceanConfig
	if err := p.GetProviderConfig("digitalocean", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetHetznerConfig is a convenience method to get Hetzner configuration
func (p *ProjectConfig) GetHetznerConfig() (*HetznerConfig, error) {
	var config HetznerConfig
	if err := p.GetProviderConfig("hetzner", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetS3Config is a convenience method to get S3 configuration
func (p *ProjectConfig) GetS3Config() (*S3Config, error) {
	var config S3Config
	if err := p.GetProviderConfig("s3", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

type Config struct {
	Projects map[string]ProjectConfig `json:"projects"`
}

func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".lightfold/config.json"
	}
	return filepath.Join(homeDir, ".lightfold", "config.json")
}

func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{Projects: make(map[string]ProjectConfig)}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Projects == nil {
		config.Projects = make(map[string]ProjectConfig)
	}

	return &config, nil
}

func (c *Config) SaveConfig() error {
	configPath := GetConfigPath()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) GetProject(projectPath string) (ProjectConfig, bool) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return ProjectConfig{}, false
	}

	project, exists := c.Projects[absPath]
	return project, exists
}

func (c *Config) SetProject(projectPath string, project ProjectConfig) error {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	c.Projects[absPath] = project
	return nil
}

// TokenConfig stores API tokens for all providers
type TokenConfig struct {
	Tokens map[string]string `json:"tokens,omitempty"` // provider_name -> token
}

func GetTokensPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".lightfold/tokens.json"
	}
	return filepath.Join(homeDir, ".lightfold", "tokens.json")
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

// SaveTokens saves API tokens to secure storage
func (t *TokenConfig) SaveTokens() error {
	tokensPath := GetTokensPath()

	// Ensure directory exists with secure permissions
	dir := filepath.Dir(tokensPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create tokens directory: %w", err)
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	// Write with secure permissions (readable only by owner)
	if err := os.WriteFile(tokensPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write tokens file: %w", err)
	}

	return nil
}

// SetToken stores an API token for a specific provider
// Automatically trims whitespace and removes surrounding brackets/quotes
func (t *TokenConfig) SetToken(provider, token string) {
	if t.Tokens == nil {
		t.Tokens = make(map[string]string)
	}

	// Keep removing outer layers of brackets, quotes, and whitespace until nothing changes
	for {
		oldToken := token

		// Trim whitespace
		token = strings.TrimSpace(token)

		// Remove surrounding brackets: [token] -> token
		token = strings.TrimPrefix(token, "[")
		token = strings.TrimSuffix(token, "]")

		// Trim whitespace again
		token = strings.TrimSpace(token)

		// Remove surrounding quotes: "token" or 'token' -> token
		token = strings.Trim(token, "\"'")

		// If nothing changed, we're done
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
