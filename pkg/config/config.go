package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

type S3Config struct {
	Bucket    string `json:"bucket"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
}

type ProjectConfig struct {
	Framework    string              `json:"framework"`
	Target       string              `json:"target"`
	DigitalOcean *DigitalOceanConfig `json:"digitalocean,omitempty"`
	S3           *S3Config           `json:"s3,omitempty"`
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

type TokenConfig struct {
	DigitalOceanToken string `json:"digitalocean_token,omitempty"`
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
		return &TokenConfig{}, nil
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokens file: %w", err)
	}

	var tokens TokenConfig
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse tokens file: %w", err)
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

// SetDigitalOceanToken stores a DigitalOcean API token
// Automatically trims whitespace and removes surrounding brackets/quotes
func (t *TokenConfig) SetDigitalOceanToken(token string) {
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

	t.DigitalOceanToken = token
}

// GetDigitalOceanToken retrieves the DigitalOcean API token
func (t *TokenConfig) GetDigitalOceanToken() string {
	return t.DigitalOceanToken
}
