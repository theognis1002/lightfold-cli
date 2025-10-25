package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

// AWSCredentials represents AWS authentication credentials
type AWSCredentials struct {
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Profile         string `json:"profile,omitempty"`
}

// parseCredentials parses AWS credentials from JSON string
// Supports both explicit credentials and AWS profile names
func parseCredentials(credsJSON string) (AWSCredentials, error) {
	var creds AWSCredentials

	// Try to parse as JSON
	if err := json.Unmarshal([]byte(credsJSON), &creds); err != nil {
		// If JSON parsing fails, treat as plain access key (for backward compatibility)
		// Check if it looks like an access key (starts with AKIA)
		if strings.HasPrefix(strings.TrimSpace(credsJSON), "AKIA") {
			return AWSCredentials{
				AccessKeyID: strings.TrimSpace(credsJSON),
			}, nil
		}

		return AWSCredentials{}, fmt.Errorf("invalid credentials format: %w", err)
	}

	// Check for AWS_PROFILE environment variable as fallback
	if creds.Profile == "" && creds.AccessKeyID == "" {
		if envProfile := os.Getenv("AWS_PROFILE"); envProfile != "" {
			creds.Profile = envProfile
		}
	}

	// If profile is specified, try to load credentials from ~/.aws/credentials
	if creds.Profile != "" && creds.AccessKeyID == "" {
		profileCreds, err := loadProfileCredentials(creds.Profile)
		if err == nil {
			creds.AccessKeyID = profileCreds.AccessKeyID
			creds.SecretAccessKey = profileCreds.SecretAccessKey
		}
	}

	// Validate that we have either profile or credentials
	if creds.Profile == "" && creds.AccessKeyID == "" {
		return AWSCredentials{}, fmt.Errorf("no AWS credentials provided (need access_key_id or profile)")
	}

	return creds, nil
}

// loadProfileCredentials loads AWS credentials from ~/.aws/credentials file
func loadProfileCredentials(profile string) (AWSCredentials, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	credentialsPath := filepath.Join(homeDir, ".aws", "credentials")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return AWSCredentials{}, fmt.Errorf("AWS credentials file not found: %s", credentialsPath)
	}

	cfg, err := ini.Load(credentialsPath)
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("failed to load AWS credentials file: %w", err)
	}

	section, err := cfg.GetSection(profile)
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("profile '%s' not found in credentials file", profile)
	}

	accessKeyID := section.Key("aws_access_key_id").String()
	secretAccessKey := section.Key("aws_secret_access_key").String()

	if accessKeyID == "" || secretAccessKey == "" {
		return AWSCredentials{}, fmt.Errorf("incomplete credentials in profile '%s'", profile)
	}

	return AWSCredentials{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Profile:         profile,
	}, nil
}

// ToJSON converts credentials to JSON string for storage
func (c *AWSCredentials) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return string(data), nil
}
