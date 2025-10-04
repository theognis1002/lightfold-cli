package hetzner

import (
	"context"
	"lightfold/pkg/providers"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func TestNewClient(t *testing.T) {
	token := "test-token"
	client := NewClient(token)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.token != token {
		t.Errorf("Expected token %s, got %s", token, client.token)
	}

	if client.client == nil {
		t.Fatal("Expected hcloud client to be initialized, got nil")
	}
}

func TestName(t *testing.T) {
	client := NewClient("test-token")
	expected := "hetzner"
	if client.Name() != expected {
		t.Errorf("Expected name %s, got %s", expected, client.Name())
	}
}

func TestDisplayName(t *testing.T) {
	client := NewClient("test-token")
	expected := "Hetzner Cloud"
	if client.DisplayName() != expected {
		t.Errorf("Expected display name %s, got %s", expected, client.DisplayName())
	}
}

func TestSupportsProvisioning(t *testing.T) {
	client := NewClient("test-token")
	if !client.SupportsProvisioning() {
		t.Error("Expected Hetzner to support provisioning")
	}
}

func TestSupportsBYOS(t *testing.T) {
	client := NewClient("test-token")
	if !client.SupportsBYOS() {
		t.Error("Expected Hetzner to support BYOS")
	}
}

func TestValidateCredentials_EmptyToken(t *testing.T) {
	client := NewClient("")
	err := client.ValidateCredentials(context.Background())

	if err == nil {
		t.Fatal("Expected error for empty token, got nil")
	}

	provErr, ok := err.(*providers.ProviderError)
	if !ok {
		t.Fatalf("Expected ProviderError, got %T", err)
	}

	if provErr.Code != "invalid_credentials" {
		t.Errorf("Expected error code 'invalid_credentials', got %s", provErr.Code)
	}
}

func TestConvertServerToProvider(t *testing.T) {
	serverID := int64(12345)
	serverName := "test-server"
	serverStatus := hcloud.ServerStatusRunning

	mockServer := &hcloud.Server{
		ID:     serverID,
		Name:   serverName,
		Status: serverStatus,
		ServerType: &hcloud.ServerType{
			Name:   "cx11",
			Cores:  1,
			Memory: 2.0,
			Disk:   20,
		},
		PublicNet: hcloud.ServerPublicNet{
			IPv4: hcloud.ServerPublicNetIPv4{
				IP: parseIP("192.168.1.100"),
			},
		},
		Datacenter: &hcloud.Datacenter{
			Name: "nbg1-dc3",
			Location: &hcloud.Location{
				Name: "nbg1",
				City: "Nuremberg",
			},
		},
		Image: &hcloud.Image{
			Name: "ubuntu-22.04",
		},
		Created: time.Now(),
		Labels:  map[string]string{"env": "test", "app": "myapp"},
		Locked:  false,
	}

	result := convertServerToProvider(mockServer)

	if result.ID != strconv.FormatInt(serverID, 10) {
		t.Errorf("Expected ID %s, got %s", strconv.FormatInt(serverID, 10), result.ID)
	}

	if result.Name != serverName {
		t.Errorf("Expected name %s, got %s", serverName, result.Name)
	}

	if result.Status != string(serverStatus) {
		t.Errorf("Expected status %s, got %s", string(serverStatus), result.Status)
	}

	if result.PublicIPv4 != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", result.PublicIPv4)
	}

	if result.Region != "nbg1" {
		t.Errorf("Expected region nbg1, got %s", result.Region)
	}

	if result.Size != "cx11" {
		t.Errorf("Expected size cx11, got %s", result.Size)
	}

	if result.Image != "ubuntu-22.04" {
		t.Errorf("Expected image ubuntu-22.04, got %s", result.Image)
	}

	if result.Metadata["vcpus"] != "1" {
		t.Errorf("Expected vcpus 1, got %s", result.Metadata["vcpus"])
	}

	if result.Metadata["memory"] != "2" {
		t.Errorf("Expected memory 2, got %s", result.Metadata["memory"])
	}

	if result.Metadata["disk"] != "20" {
		t.Errorf("Expected disk 20, got %s", result.Metadata["disk"])
	}

	if result.Metadata["locked"] != "false" {
		t.Errorf("Expected locked false, got %s", result.Metadata["locked"])
	}

	expectedTags := 2
	if len(result.Tags) != expectedTags {
		t.Errorf("Expected %d tags, got %d", expectedTags, len(result.Tags))
	}
}

func TestConvertTagsToLabels(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected map[string]string
	}{
		{
			name:     "key-value tags",
			tags:     []string{"env:production", "app:myapp"},
			expected: map[string]string{"env": "production", "app": "myapp"},
		},
		{
			name:     "simple tags",
			tags:     []string{"production", "critical"},
			expected: map[string]string{"production": "true", "critical": "true"},
		},
		{
			name:     "mixed tags",
			tags:     []string{"env:prod", "critical"},
			expected: map[string]string{"env": "prod", "critical": "true"},
		},
		{
			name:     "empty tags",
			tags:     []string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTagsToLabels(tt.tags)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d labels, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("Expected label %s=%s, got %s", key, expectedValue, result[key])
				}
			}
		})
	}
}

func TestExtractVersionFromImageName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Ubuntu 22.04",
			input:    "Ubuntu 22.04",
			expected: "22.04",
		},
		{
			name:     "Ubuntu 20.04 LTS",
			input:    "Ubuntu 20.04 LTS",
			expected: "20.04",
		},
		{
			name:     "Debian 11.5",
			input:    "Debian 11.5",
			expected: "11.5",
		},
		{
			name:     "No version",
			input:    "Ubuntu Server",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromImageName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected version %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestProviderRegistration(t *testing.T) {
	provider, err := providers.GetProvider("hetzner", "test-token")
	if err != nil {
		t.Fatalf("Expected Hetzner provider to be registered, got error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider instance, got nil")
	}

	if provider.Name() != "hetzner" {
		t.Errorf("Expected provider name 'hetzner', got %s", provider.Name())
	}
}

func parseIP(ip string) net.IP {
	return net.ParseIP(ip)
}
