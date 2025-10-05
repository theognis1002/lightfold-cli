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

// TestConvertServerToProvider_EdgeCases tests edge cases in server conversion
func TestConvertServerToProvider_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		server *hcloud.Server
		checks func(*testing.T, *providers.Server)
	}{
		{
			name: "server with nil datacenter",
			server: &hcloud.Server{
				ID:     12345,
				Name:   "test-server",
				Status: hcloud.ServerStatusRunning,
				ServerType: &hcloud.ServerType{
					Name:   "cpx11",
					Cores:  2,
					Memory: 2.0,
					Disk:   40,
				},
				PublicNet: hcloud.ServerPublicNet{
					IPv4: hcloud.ServerPublicNetIPv4{
						IP: parseIP("1.2.3.4"),
					},
				},
				Datacenter: nil,
				Image: &hcloud.Image{
					Name: "ubuntu-22.04",
				},
				Created: time.Now(),
				Labels:  map[string]string{},
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.Region != "" {
					t.Errorf("Expected empty region, got %s", s.Region)
				}
				if s.Metadata["datacenter"] != "" {
					t.Errorf("Expected empty datacenter metadata, got %s", s.Metadata["datacenter"])
				}
			},
		},
		{
			name: "server with nil image",
			server: &hcloud.Server{
				ID:     12345,
				Name:   "test-server",
				Status: hcloud.ServerStatusRunning,
				ServerType: &hcloud.ServerType{
					Name:   "cpx11",
					Cores:  2,
					Memory: 2.0,
					Disk:   40,
				},
				PublicNet: hcloud.ServerPublicNet{
					IPv4: hcloud.ServerPublicNetIPv4{
						IP: parseIP("1.2.3.4"),
					},
				},
				Datacenter: &hcloud.Datacenter{
					Name: "nbg1-dc3",
					Location: &hcloud.Location{
						Name: "nbg1",
						City: "Nuremberg",
					},
				},
				Image:   nil,
				Created: time.Now(),
				Labels:  map[string]string{},
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.Image != "" {
					t.Errorf("Expected empty image, got %s", s.Image)
				}
			},
		},
		{
			name: "server with private network",
			server: &hcloud.Server{
				ID:     12345,
				Name:   "test-server",
				Status: hcloud.ServerStatusRunning,
				ServerType: &hcloud.ServerType{
					Name:   "cpx11",
					Cores:  2,
					Memory: 2.0,
					Disk:   40,
				},
				PublicNet: hcloud.ServerPublicNet{
					IPv4: hcloud.ServerPublicNetIPv4{
						IP: parseIP("1.2.3.4"),
					},
				},
				PrivateNet: []hcloud.ServerPrivateNet{
					{
						IP: parseIP("10.0.0.5"),
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
				Labels:  map[string]string{},
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.PrivateIPv4 != "10.0.0.5" {
					t.Errorf("Expected private IP 10.0.0.5, got %s", s.PrivateIPv4)
				}
			},
		},
		{
			name: "server with locked status",
			server: &hcloud.Server{
				ID:     12345,
				Name:   "test-server",
				Status: hcloud.ServerStatusRunning,
				ServerType: &hcloud.ServerType{
					Name:   "cpx11",
					Cores:  2,
					Memory: 2.0,
					Disk:   40,
				},
				PublicNet: hcloud.ServerPublicNet{
					IPv4: hcloud.ServerPublicNetIPv4{
						IP: parseIP("1.2.3.4"),
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
				Labels:  map[string]string{},
				Locked:  true,
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.Metadata["locked"] != "true" {
					t.Errorf("Expected locked true, got %s", s.Metadata["locked"])
				}
			},
		},
		{
			name: "server with complex labels",
			server: &hcloud.Server{
				ID:     12345,
				Name:   "test-server",
				Status: hcloud.ServerStatusRunning,
				ServerType: &hcloud.ServerType{
					Name:   "cpx11",
					Cores:  2,
					Memory: 2.0,
					Disk:   40,
				},
				PublicNet: hcloud.ServerPublicNet{
					IPv4: hcloud.ServerPublicNetIPv4{
						IP: parseIP("1.2.3.4"),
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
				Labels: map[string]string{
					"env":     "production",
					"project": "lightfold",
					"managed": "true",
				},
			},
			checks: func(t *testing.T, s *providers.Server) {
				if len(s.Tags) != 3 {
					t.Errorf("Expected 3 tags, got %d", len(s.Tags))
				}
				hasEnvTag := false
				for _, tag := range s.Tags {
					if tag == "env:production" {
						hasEnvTag = true
						break
					}
				}
				if !hasEnvTag {
					t.Error("Expected env:production tag not found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertServerToProvider(tt.server)
			if result == nil {
				t.Fatal("Expected non-nil server, got nil")
			}
			tt.checks(t, result)
		})
	}
}

// TestExtractVersionFromImageName_EdgeCases tests additional edge cases
func TestExtractVersionFromImageName_EdgeCases(t *testing.T) {
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
		{
			name:     "Version with more digits (7 chars, exceeds 6 char limit)",
			input:    "Ubuntu 24.04.1",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only numbers",
			input:    "22.04",
			expected: "22.04",
		},
		{
			name:     "Long version number (should be skipped)",
			input:    "Ubuntu 22.04.1234",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromImageName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected version %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestConvertTagsToLabels_EdgeCases tests additional edge cases
func TestConvertTagsToLabels_EdgeCases(t *testing.T) {
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
		{
			name:     "tags with multiple colons",
			tags:     []string{"url:https://example.com"},
			expected: map[string]string{"url": "https://example.com"},
		},
		{
			name:     "tags with empty values",
			tags:     []string{"key:", "emptykey"},
			expected: map[string]string{"key": "", "emptykey": "true"},
		},
		{
			name:     "duplicate keys (last wins)",
			tags:     []string{"env:dev", "env:prod"},
			expected: map[string]string{"env": "prod"},
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
					t.Errorf("Expected label %s=%q, got %q", key, expectedValue, result[key])
				}
			}
		})
	}
}

// TestValidateCredentials_ErrorCases tests various credential validation error cases
func TestValidateCredentials_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		expectedCode  string
		shouldHaveErr bool
	}{
		{
			name:          "empty token",
			token:         "",
			expectedCode:  "invalid_credentials",
			shouldHaveErr: true,
		},
		{
			name:          "whitespace token",
			token:         "   ",
			expectedCode:  "",
			shouldHaveErr: false,
		},
		{
			name:          "valid format token",
			token:         "test-token-12345",
			expectedCode:  "",
			shouldHaveErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.token)
			err := client.ValidateCredentials(context.Background())

			if tt.shouldHaveErr && err == nil {
				t.Fatal("Expected error, got nil")
			}

			if tt.shouldHaveErr {
				provErr, ok := err.(*providers.ProviderError)
				if !ok {
					t.Fatalf("Expected ProviderError, got %T", err)
				}

				if provErr.Code != tt.expectedCode {
					t.Errorf("Expected error code %q, got %q", tt.expectedCode, provErr.Code)
				}

				if provErr.Provider != "hetzner" {
					t.Errorf("Expected provider 'hetzner', got %q", provErr.Provider)
				}
			}
		})
	}
}

// TestProviderInterface ensures Client implements the Provider interface
func TestProviderInterface(t *testing.T) {
	var _ providers.Provider = (*Client)(nil)
}

// TestClientCreationWithValidToken tests client creation with various token formats
func TestClientCreationWithValidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "standard token",
			token: "test-token",
		},
		{
			name:  "long token",
			token: "very-long-token-with-many-characters-1234567890",
		},
		{
			name:  "token with special chars",
			token: "token-with_underscores.and.dots",
		},
		{
			name:  "empty token (should still create client)",
			token: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.token)

			if client == nil {
				t.Fatal("Expected client to be created, got nil")
			}

			if client.token != tt.token {
				t.Errorf("Expected token %q, got %q", tt.token, client.token)
			}

			if client.client == nil {
				t.Fatal("Expected hcloud client to be initialized, got nil")
			}

			if client.Name() != "hetzner" {
				t.Errorf("Expected name 'hetzner', got %q", client.Name())
			}

			if client.DisplayName() != "Hetzner Cloud" {
				t.Errorf("Expected display name 'Hetzner Cloud', got %q", client.DisplayName())
			}

			if !client.SupportsProvisioning() {
				t.Error("Expected to support provisioning")
			}

			if !client.SupportsBYOS() {
				t.Error("Expected to support BYOS")
			}
		})
	}
}
