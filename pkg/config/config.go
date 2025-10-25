package config

import (
	"encoding/json"
	"fmt"
	"lightfold/pkg/providers"
	"os"
	"path/filepath"
	"strings"
)

type ProviderConfig interface {
	GetIP() string
	GetUsername() string
	GetSSHKey() string
	IsProvisioned() bool
	GetServerID() string // Returns the cloud provider's server/instance/machine ID
}

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

func (d *DigitalOceanConfig) GetIP() string       { return d.IP }
func (d *DigitalOceanConfig) GetUsername() string { return d.Username }
func (d *DigitalOceanConfig) GetSSHKey() string   { return d.SSHKey }
func (d *DigitalOceanConfig) IsProvisioned() bool { return d.Provisioned }
func (d *DigitalOceanConfig) GetServerID() string { return d.DropletID }

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

func (h *HetznerConfig) GetIP() string       { return h.IP }
func (h *HetznerConfig) GetUsername() string { return h.Username }
func (h *HetznerConfig) GetSSHKey() string   { return h.SSHKey }
func (h *HetznerConfig) IsProvisioned() bool { return h.Provisioned }
func (h *HetznerConfig) GetServerID() string { return h.ServerID }

type VultrConfig struct {
	InstanceID  string `json:"instance_id,omitempty"` // For provisioned instances
	IP          string `json:"ip"`
	SSHKey      string `json:"ssh_key"`
	SSHKeyName  string `json:"ssh_key_name,omitempty"`
	Username    string `json:"username"`
	Region      string `json:"region,omitempty"`
	Plan        string `json:"plan,omitempty"` // Vultr uses "plan" instead of "size"
	Provisioned bool   `json:"provisioned,omitempty"`
}

func (v *VultrConfig) GetIP() string       { return v.IP }
func (v *VultrConfig) GetUsername() string { return v.Username }
func (v *VultrConfig) GetSSHKey() string   { return v.SSHKey }
func (v *VultrConfig) IsProvisioned() bool { return v.Provisioned }
func (v *VultrConfig) GetServerID() string { return v.InstanceID }

type FlyioConfig struct {
	MachineID      string `json:"machine_id,omitempty"`
	AppName        string `json:"app_name,omitempty"`        // fly.io requires app context
	OrganizationID string `json:"organization_id,omitempty"` // fly.io organization ID
	IP             string `json:"ip"`
	SSHKey         string `json:"ssh_key"`
	SSHKeyName     string `json:"ssh_key_name,omitempty"`
	Username       string `json:"username"`
	Region         string `json:"region,omitempty"`
	Size           string `json:"size,omitempty"`
	Provisioned    bool   `json:"provisioned,omitempty"`
}

func (f *FlyioConfig) GetIP() string       { return f.IP }
func (f *FlyioConfig) GetUsername() string { return f.Username }
func (f *FlyioConfig) GetSSHKey() string   { return f.SSHKey }
func (f *FlyioConfig) IsProvisioned() bool { return f.Provisioned }
func (f *FlyioConfig) GetServerID() string { return f.MachineID }

type LinodeConfig struct {
	InstanceID  string `json:"instance_id,omitempty"` // For provisioned instances
	IP          string `json:"ip"`
	SSHKey      string `json:"ssh_key"`
	SSHKeyName  string `json:"ssh_key_name,omitempty"`
	Username    string `json:"username"`
	Region      string `json:"region,omitempty"`
	Plan        string `json:"plan,omitempty"` // Linode uses "plan" or "type"
	Provisioned bool   `json:"provisioned,omitempty"`
	RootPass    string `json:"root_pass,omitempty"` // Generated root password for emergency access
}

func (l *LinodeConfig) GetIP() string       { return l.IP }
func (l *LinodeConfig) GetUsername() string { return l.Username }
func (l *LinodeConfig) GetSSHKey() string   { return l.SSHKey }
func (l *LinodeConfig) IsProvisioned() bool { return l.Provisioned }
func (l *LinodeConfig) GetServerID() string { return l.InstanceID }

type AWSConfig struct {
	InstanceID      string `json:"instance_id,omitempty"` // EC2 instance ID
	IP              string `json:"ip"`
	SSHKey          string `json:"ssh_key"`
	SSHKeyName      string `json:"ssh_key_name,omitempty"`
	Username        string `json:"username"`
	Region          string `json:"region,omitempty"`
	InstanceType    string `json:"instance_type,omitempty"` // e.g., "t3.small"
	Provisioned     bool   `json:"provisioned,omitempty"`
	ElasticIP       string `json:"elastic_ip,omitempty"`        // Allocation ID if EIP used
	SecurityGroupID string `json:"security_group_id,omitempty"` // Security group ID for cleanup
	VpcID           string `json:"vpc_id,omitempty"`            // VPC ID
	SubnetID        string `json:"subnet_id,omitempty"`         // Subnet ID
}

func (a *AWSConfig) GetIP() string       { return a.IP }
func (a *AWSConfig) GetUsername() string { return a.Username }
func (a *AWSConfig) GetSSHKey() string   { return a.SSHKey }
func (a *AWSConfig) IsProvisioned() bool { return a.Provisioned }
func (a *AWSConfig) GetServerID() string { return a.InstanceID }

type S3Config struct {
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

func (s *S3Config) GetIP() string       { return "" }
func (s *S3Config) GetUsername() string { return "" }
func (s *S3Config) GetSSHKey() string   { return "" }
func (s *S3Config) IsProvisioned() bool { return false }
func (s *S3Config) GetServerID() string { return "" }

type DeploymentOptions struct {
	SkipBuild     bool              `json:"skip_build,omitempty"`
	EnvVars       map[string]string `json:"env_vars,omitempty"`
	BuildCommand  string            `json:"build_command,omitempty"`
	RunCommand    string            `json:"run_command,omitempty"`
	BuildCommands []string          `json:"build_commands,omitempty"`
	RunCommands   []string          `json:"run_commands,omitempty"`
}

type DomainConfig struct {
	Domain     string `json:"domain,omitempty"`      // Full domain: app.example.com
	RootDomain string `json:"root_domain,omitempty"` // Root domain: example.com
	Subdomain  string `json:"subdomain,omitempty"`   // Subdomain: app
	SSLEnabled bool   `json:"ssl_enabled,omitempty"`
	SSLManager string `json:"ssl_manager,omitempty"` // "certbot", "caddy", etc.
	ProxyType  string `json:"proxy_type,omitempty"`  // "nginx", "caddy", etc.
	Email      string `json:"email,omitempty"`       // Email for SSL certificate registration
}

type TargetConfig struct {
	ProjectPath    string                     `json:"project_path"`
	Framework      string                     `json:"framework"`
	Provider       string                     `json:"provider"`
	Builder        string                     `json:"builder,omitempty"`
	ServerIP       string                     `json:"server_ip,omitempty"`
	Port           int                        `json:"port,omitempty"`
	ProviderConfig map[string]json.RawMessage `json:"provider_config"`
	Deploy         *DeploymentOptions         `json:"deploy,omitempty"`
	Domain         *DomainConfig              `json:"domain,omitempty"`
}

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

func (t *TargetConfig) GetDigitalOceanConfig() (*DigitalOceanConfig, error) {
	var config DigitalOceanConfig
	if err := t.GetProviderConfig("digitalocean", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetHetznerConfig() (*HetznerConfig, error) {
	var config HetznerConfig
	if err := t.GetProviderConfig("hetzner", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetS3Config() (*S3Config, error) {
	var config S3Config
	if err := t.GetProviderConfig("s3", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetVultrConfig() (*VultrConfig, error) {
	var config VultrConfig
	if err := t.GetProviderConfig("vultr", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetFlyioConfig() (*FlyioConfig, error) {
	var config FlyioConfig
	if err := t.GetProviderConfig("flyio", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetLinodeConfig() (*LinodeConfig, error) {
	var config LinodeConfig
	if err := t.GetProviderConfig("linode", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetAWSConfig() (*AWSConfig, error) {
	var config AWSConfig
	if err := t.GetProviderConfig("aws", &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (t *TargetConfig) GetSSHProviderConfig() (ProviderConfig, error) {
	switch t.Provider {
	case "digitalocean":
		return t.GetDigitalOceanConfig()
	case "hetzner":
		return t.GetHetznerConfig()
	case "vultr":
		return t.GetVultrConfig()
	case "flyio":
		return t.GetFlyioConfig()
	case "linode":
		return t.GetLinodeConfig()
	case "aws":
		return t.GetAWSConfig()
	case "s3":
		return nil, fmt.Errorf("S3 is not an SSH-based provider")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", t.Provider)
	}
}

func (t *TargetConfig) GetAnyProviderConfig() (ProviderConfig, error) {
	switch t.Provider {
	case "digitalocean":
		return t.GetDigitalOceanConfig()
	case "hetzner":
		return t.GetHetznerConfig()
	case "vultr":
		return t.GetVultrConfig()
	case "flyio":
		return t.GetFlyioConfig()
	case "linode":
		return t.GetLinodeConfig()
	case "aws":
		return t.GetAWSConfig()
	case "s3":
		return t.GetS3Config()
	default:
		return nil, fmt.Errorf("unsupported provider: %s", t.Provider)
	}
}

// RequiresSSHDeployment returns true if this target uses SSH-based deployment
// Uses the provider's SupportsSSH() method for polymorphic dispatch
func (t *TargetConfig) RequiresSSHDeployment() bool {
	if t.Provider == "s3" {
		return false
	}

	tokens, err := LoadTokens()
	if err != nil {
		return true
	}

	token := tokens.GetToken(t.Provider)
	if token == "" {
		return true
	}

	provider, err := providers.GetProvider(t.Provider, token)
	if err != nil {
		return true
	}

	return provider.SupportsSSH()
}

type Config struct {
	Targets     map[string]TargetConfig `json:"targets"`
	NumReleases int                     `json:"keep_releases,omitempty"`
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
			Targets:     make(map[string]TargetConfig),
			NumReleases: DefaultNumReleases,
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

	if config.NumReleases == 0 {
		config.NumReleases = DefaultNumReleases
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

func (c *Config) GetTarget(targetName string) (TargetConfig, bool) {
	target, exists := c.Targets[targetName]
	return target, exists
}

func (c *Config) SetTarget(targetName string, target TargetConfig) error {
	c.Targets[targetName] = target
	return nil
}

func (c *Config) DeleteTarget(targetName string) error {
	if _, exists := c.Targets[targetName]; !exists {
		return nil
	}

	delete(c.Targets, targetName)
	return c.SaveConfig()
}

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

// GetTargetsByServerIP returns all targets deployed to a specific server
func (c *Config) GetTargetsByServerIP(serverIP string) map[string]TargetConfig {
	targets := make(map[string]TargetConfig)
	for name, target := range c.Targets {
		if target.ServerIP == serverIP {
			targets[name] = target
		}
	}
	return targets
}

type TokenConfig map[string]string

func GetTokensPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(LocalConfigDir, LocalTokensFile)
	}
	return filepath.Join(homeDir, LocalConfigDir, LocalTokensFile)
}

func LoadTokens() (TokenConfig, error) {
	tokensPath := GetTokensPath()

	dir := filepath.Dir(tokensPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create tokens directory: %w", err)
	}

	if _, err := os.Stat(tokensPath); os.IsNotExist(err) {
		return make(TokenConfig), nil
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokens file: %w", err)
	}

	var tokens TokenConfig
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse tokens file: %w", err)
	}

	if tokens == nil {
		tokens = make(TokenConfig)
	}

	return tokens, nil
}

func (t TokenConfig) SaveTokens() error {
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

func (t TokenConfig) SetToken(provider, token string) {
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

	t[provider] = token
}

func (t TokenConfig) GetToken(provider string) string {
	return t[provider]
}

func (t TokenConfig) HasToken(provider string) bool {
	return t.GetToken(provider) != ""
}

func (t TokenConfig) SetDigitalOceanToken(token string) {
	t.SetToken("digitalocean", token)
}

func (t TokenConfig) GetDigitalOceanToken() string {
	return t.GetToken("digitalocean")
}
