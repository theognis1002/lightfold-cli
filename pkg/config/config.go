package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DigitalOceanConfig struct {
	IP       string `json:"ip"`
	SSHKey   string `json:"ssh_key"`
	Username string `json:"username"`
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

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// If config file doesn't exist, return empty config
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