package config

import (
	"encoding/json"
	"testing"
)

// TestAWSConfigImplementsProviderConfig verifies AWSConfig implements ProviderConfig interface
func TestAWSConfigImplementsProviderConfig(t *testing.T) {
	var _ ProviderConfig = &AWSConfig{}
}

// TestAWSConfigGetters verifies all ProviderConfig interface methods work correctly
func TestAWSConfigGetters(t *testing.T) {
	awsConfig := &AWSConfig{
		InstanceID:   "i-1234567890abcdef0",
		IP:           "203.0.113.42",
		SSHKey:       "/home/user/.ssh/id_rsa",
		SSHKeyName:   "lightfold-key",
		Username:     "ubuntu",
		Region:       "us-east-1",
		InstanceType: "t3.small",
		Provisioned:  true,
		ElasticIP:    "eipalloc-12345678",
	}

	// Test GetIP
	if got := awsConfig.GetIP(); got != "203.0.113.42" {
		t.Errorf("GetIP() = %v, want %v", got, "203.0.113.42")
	}

	// Test GetUsername
	if got := awsConfig.GetUsername(); got != "ubuntu" {
		t.Errorf("GetUsername() = %v, want %v", got, "ubuntu")
	}

	// Test GetSSHKey
	if got := awsConfig.GetSSHKey(); got != "/home/user/.ssh/id_rsa" {
		t.Errorf("GetSSHKey() = %v, want %v", got, "/home/user/.ssh/id_rsa")
	}

	// Test IsProvisioned
	if got := awsConfig.IsProvisioned(); got != true {
		t.Errorf("IsProvisioned() = %v, want %v", got, true)
	}

	// Test GetServerID
	if got := awsConfig.GetServerID(); got != "i-1234567890abcdef0" {
		t.Errorf("GetServerID() = %v, want %v", got, "i-1234567890abcdef0")
	}
}

// TestTargetConfigGetAWSConfig verifies GetAWSConfig helper method
func TestTargetConfigGetAWSConfig(t *testing.T) {
	target := &TargetConfig{
		ProjectPath:    "/path/to/project",
		Framework:      "Next.js",
		Provider:       "aws",
		ProviderConfig: make(map[string]json.RawMessage),
	}

	awsConfig := &AWSConfig{
		InstanceID:   "i-1234567890abcdef0",
		IP:           "203.0.113.42",
		SSHKey:       "/home/user/.ssh/id_rsa",
		Username:     "ubuntu",
		Region:       "us-east-1",
		InstanceType: "t3.small",
		Provisioned:  true,
	}

	// Set AWS config
	if err := target.SetProviderConfig("aws", awsConfig); err != nil {
		t.Fatalf("SetProviderConfig() error = %v", err)
	}

	// Get AWS config
	retrievedConfig, err := target.GetAWSConfig()
	if err != nil {
		t.Fatalf("GetAWSConfig() error = %v", err)
	}

	if retrievedConfig.InstanceID != "i-1234567890abcdef0" {
		t.Errorf("InstanceID = %v, want %v", retrievedConfig.InstanceID, "i-1234567890abcdef0")
	}

	if retrievedConfig.IP != "203.0.113.42" {
		t.Errorf("IP = %v, want %v", retrievedConfig.IP, "203.0.113.42")
	}

	if retrievedConfig.Region != "us-east-1" {
		t.Errorf("Region = %v, want %v", retrievedConfig.Region, "us-east-1")
	}

	if retrievedConfig.InstanceType != "t3.small" {
		t.Errorf("InstanceType = %v, want %v", retrievedConfig.InstanceType, "t3.small")
	}
}

// TestTargetConfigGetSSHProviderConfigAWS verifies GetSSHProviderConfig works for AWS
func TestTargetConfigGetSSHProviderConfigAWS(t *testing.T) {
	target := &TargetConfig{
		ProjectPath:    "/path/to/project",
		Framework:      "Next.js",
		Provider:       "aws",
		ProviderConfig: make(map[string]json.RawMessage),
	}

	awsConfig := &AWSConfig{
		InstanceID:  "i-1234567890abcdef0",
		IP:          "203.0.113.42",
		SSHKey:      "/home/user/.ssh/id_rsa",
		Username:    "ubuntu",
		Provisioned: true,
	}

	if err := target.SetProviderConfig("aws", awsConfig); err != nil {
		t.Fatalf("SetProviderConfig() error = %v", err)
	}

	providerConfig, err := target.GetSSHProviderConfig()
	if err != nil {
		t.Fatalf("GetSSHProviderConfig() error = %v", err)
	}

	if providerConfig.GetIP() != "203.0.113.42" {
		t.Errorf("GetIP() = %v, want %v", providerConfig.GetIP(), "203.0.113.42")
	}

	if providerConfig.GetUsername() != "ubuntu" {
		t.Errorf("GetUsername() = %v, want %v", providerConfig.GetUsername(), "ubuntu")
	}

	if providerConfig.GetServerID() != "i-1234567890abcdef0" {
		t.Errorf("GetServerID() = %v, want %v", providerConfig.GetServerID(), "i-1234567890abcdef0")
	}
}

// TestAWSElasticIPMarker verifies Elastic IP allocation marker
func TestAWSElasticIPMarker(t *testing.T) {
	tests := []struct {
		name          string
		elasticIP     string
		wantAllocated bool
	}{
		{
			name:          "with allocate marker",
			elasticIP:     "allocate",
			wantAllocated: true,
		},
		{
			name:          "with allocation ID",
			elasticIP:     "eipalloc-12345678",
			wantAllocated: true,
		},
		{
			name:          "without Elastic IP",
			elasticIP:     "",
			wantAllocated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			awsConfig := &AWSConfig{
				ElasticIP: tt.elasticIP,
			}

			hasElasticIP := awsConfig.ElasticIP != ""
			if hasElasticIP != tt.wantAllocated {
				t.Errorf("ElasticIP presence = %v, want %v", hasElasticIP, tt.wantAllocated)
			}
		})
	}
}
