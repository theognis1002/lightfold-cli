package linode

import (
	"encoding/base64"
	"lightfold/pkg/providers"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/linode/linodego"
)

func parseIP(ipStr string) *net.IP {
	ip := net.ParseIP(ipStr)
	return &ip
}

func TestNewClient(t *testing.T) {
	token := "test-token"
	client := NewClient(token)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.client == nil {
		t.Fatal("Expected linodego client to be initialized, got nil")
	}
}

func TestName(t *testing.T) {
	client := NewClient("test-token")
	expected := "linode"
	if client.Name() != expected {
		t.Errorf("Expected name %s, got %s", expected, client.Name())
	}
}

func TestDisplayName(t *testing.T) {
	client := NewClient("test-token")
	expected := "Linode"
	if client.DisplayName() != expected {
		t.Errorf("Expected display name %s, got %s", expected, client.DisplayName())
	}
}

func TestSupportsProvisioning(t *testing.T) {
	client := NewClient("test-token")
	if !client.SupportsProvisioning() {
		t.Error("Expected Linode to support provisioning")
	}
}

func TestSupportsBYOS(t *testing.T) {
	client := NewClient("test-token")
	if !client.SupportsBYOS() {
		t.Error("Expected Linode to support BYOS")
	}
}

func TestSupportsSSH(t *testing.T) {
	client := NewClient("test-token")
	if !client.SupportsSSH() {
		t.Error("Expected Linode to support SSH")
	}
}

func TestConvertInstanceToServer(t *testing.T) {
	instanceID := 12345
	instanceName := "test-instance"
	instanceStatus := linodego.InstanceRunning
	createdTime := time.Now()

	mockInstance := &linodego.Instance{
		ID:      instanceID,
		Label:   instanceName,
		Status:  instanceStatus,
		Type:    "g6-nanode-1",
		Region:  "us-east",
		Image:   "linode/ubuntu22.04",
		IPv4:    []*net.IP{parseIP("192.168.1.100")},
		Tags:    []string{"production", "web"},
		Created: &createdTime,
		Specs: &linodego.InstanceSpec{
			VCPUs:    1,
			Memory:   1024,
			Disk:     25600,
			Transfer: 1000,
		},
		Hypervisor: "kvm",
	}

	result := convertInstanceToServer(mockInstance)

	if result.ID != intToString(instanceID) {
		t.Errorf("Expected ID %s, got %s", intToString(instanceID), result.ID)
	}

	if result.Name != instanceName {
		t.Errorf("Expected name %s, got %s", instanceName, result.Name)
	}

	if result.Status != string(instanceStatus) {
		t.Errorf("Expected status %s, got %s", string(instanceStatus), result.Status)
	}

	if result.PublicIPv4 != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", result.PublicIPv4)
	}

	if result.Region != "us-east" {
		t.Errorf("Expected region us-east, got %s", result.Region)
	}

	if result.Size != "g6-nanode-1" {
		t.Errorf("Expected size g6-nanode-1, got %s", result.Size)
	}

	if result.Image != "linode/ubuntu22.04" {
		t.Errorf("Expected image linode/ubuntu22.04, got %s", result.Image)
	}

	if result.Metadata["vcpus"] != "1" {
		t.Errorf("Expected vcpus 1, got %s", result.Metadata["vcpus"])
	}

	if result.Metadata["memory"] != "1024" {
		t.Errorf("Expected memory 1024, got %s", result.Metadata["memory"])
	}

	if result.Metadata["disk"] != "25600" {
		t.Errorf("Expected disk 25600, got %s", result.Metadata["disk"])
	}

	if result.Metadata["transfer"] != "1000" {
		t.Errorf("Expected transfer 1000, got %s", result.Metadata["transfer"])
	}

	if result.Metadata["hypervisor"] != "kvm" {
		t.Errorf("Expected hypervisor kvm, got %s", result.Metadata["hypervisor"])
	}

	expectedTags := 2
	if len(result.Tags) != expectedTags {
		t.Errorf("Expected %d tags, got %d", expectedTags, len(result.Tags))
	}
}

func TestConvertInstanceToServer_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		instance *linodego.Instance
		checks   func(*testing.T, *providers.Server)
	}{
		{
			name: "instance with no IPv4",
			instance: &linodego.Instance{
				ID:      12345,
				Label:   "test-instance",
				Status:  linodego.InstanceRunning,
				Type:    "g6-nanode-1",
				Region:  "us-east",
				Image:   "linode/ubuntu22.04",
				IPv4:    []*net.IP{},
				Created: func() *time.Time { t := time.Now(); return &t }(),
				Specs: &linodego.InstanceSpec{
					VCPUs:  1,
					Memory: 1024,
					Disk:   25600,
				},
				Hypervisor: "kvm",
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.PublicIPv4 != "" {
					t.Errorf("Expected empty IP, got %s", s.PublicIPv4)
				}
			},
		},
		{
			name: "instance with multiple IPv4 (uses first)",
			instance: &linodego.Instance{
				ID:      12345,
				Label:   "test-instance",
				Status:  linodego.InstanceRunning,
				Type:    "g6-standard-2",
				Region:  "us-west",
				Image:   "linode/ubuntu22.04",
				IPv4:    []*net.IP{parseIP("1.2.3.4"), parseIP("5.6.7.8")},
				Created: func() *time.Time { t := time.Now(); return &t }(),
				Specs: &linodego.InstanceSpec{
					VCPUs:  2,
					Memory: 4096,
					Disk:   81920,
				},
				Hypervisor: "kvm",
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.PublicIPv4 != "1.2.3.4" {
					t.Errorf("Expected IP 1.2.3.4, got %s", s.PublicIPv4)
				}
			},
		},
		{
			name: "instance with no tags",
			instance: &linodego.Instance{
				ID:      12345,
				Label:   "test-instance",
				Status:  linodego.InstanceRunning,
				Type:    "g6-nanode-1",
				Region:  "eu-west",
				Image:   "linode/ubuntu24.04",
				IPv4:    []*net.IP{parseIP("10.0.0.1")},
				Tags:    []string{},
				Created: func() *time.Time { t := time.Now(); return &t }(),
				Specs: &linodego.InstanceSpec{
					VCPUs:  1,
					Memory: 1024,
					Disk:   25600,
				},
				Hypervisor: "kvm",
			},
			checks: func(t *testing.T, s *providers.Server) {
				if len(s.Tags) != 0 {
					t.Errorf("Expected 0 tags, got %d", len(s.Tags))
				}
			},
		},
		{
			name: "instance with zero transfer (not included in metadata)",
			instance: &linodego.Instance{
				ID:      12345,
				Label:   "test-instance",
				Status:  linodego.InstanceRunning,
				Type:    "g6-nanode-1",
				Region:  "us-east",
				Image:   "linode/ubuntu22.04",
				IPv4:    []*net.IP{parseIP("192.168.1.1")},
				Created: func() *time.Time { t := time.Now(); return &t }(),
				Specs: &linodego.InstanceSpec{
					VCPUs:    1,
					Memory:   1024,
					Disk:     25600,
					Transfer: 0,
				},
				Hypervisor: "kvm",
			},
			checks: func(t *testing.T, s *providers.Server) {
				if _, exists := s.Metadata["transfer"]; exists {
					t.Error("Expected transfer not to be in metadata when zero")
				}
			},
		},
		{
			name: "instance with non-zero transfer",
			instance: &linodego.Instance{
				ID:      12345,
				Label:   "test-instance",
				Status:  linodego.InstanceRunning,
				Type:    "g6-nanode-1",
				Region:  "us-east",
				Image:   "linode/ubuntu22.04",
				IPv4:    []*net.IP{parseIP("192.168.1.1")},
				Created: func() *time.Time { t := time.Now(); return &t }(),
				Specs: &linodego.InstanceSpec{
					VCPUs:    1,
					Memory:   1024,
					Disk:     25600,
					Transfer: 2000,
				},
				Hypervisor: "kvm",
			},
			checks: func(t *testing.T, s *providers.Server) {
				if s.Metadata["transfer"] != "2000" {
					t.Errorf("Expected transfer 2000, got %s", s.Metadata["transfer"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInstanceToServer(tt.instance)
			if result == nil {
				t.Fatal("Expected non-nil server, got nil")
			}
			tt.checks(t, result)
		})
	}
}

func TestExtractVersionFromLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Ubuntu 22.04 LTS",
			input:    "Ubuntu 22.04 LTS",
			expected: "22.04",
		},
		{
			name:     "Ubuntu 20.04",
			input:    "Ubuntu 20.04",
			expected: "20.04",
		},
		{
			name:     "Ubuntu 24.04 LTS",
			input:    "Ubuntu 24.04 LTS",
			expected: "24.04",
		},
		{
			name:     "No version",
			input:    "Ubuntu Server",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only version number",
			input:    "22.04",
			expected: "22.04",
		},
		{
			name:     "Version with trailing comma",
			input:    "Ubuntu 22.04, LTS",
			expected: "22.04",
		},
		{
			name:     "Long version (exceeds 6 chars)",
			input:    "Ubuntu 22.04.1234",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromLabel(tt.input)
			if result != tt.expected {
				t.Errorf("Expected version %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("stringToInt", func(t *testing.T) {
		tests := []struct {
			input       string
			expected    int
			shouldError bool
		}{
			{"123", 123, false},
			{"0", 0, false},
			{"-456", -456, false},
			{"abc", 0, true},
			{"", 0, true},
			{"12.34", 0, true},
		}

		for _, tt := range tests {
			result, err := stringToInt(tt.input)
			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input %q, got %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		}
	})

	t.Run("intToString", func(t *testing.T) {
		tests := []struct {
			input    int
			expected string
		}{
			{123, "123"},
			{0, "0"},
			{-456, "-456"},
			{999999, "999999"},
		}

		for _, tt := range tests {
			result := intToString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		}
	})

}

func TestProviderRegistration(t *testing.T) {
	provider, err := providers.GetProvider("linode", "test-token")
	if err != nil {
		t.Fatalf("Expected Linode provider to be registered, got error: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected provider instance, got nil")
	}

	if provider.Name() != "linode" {
		t.Errorf("Expected provider name 'linode', got %s", provider.Name())
	}
}

func TestProviderInterface(t *testing.T) {
	var _ providers.Provider = (*Client)(nil)
}

func TestClientCreationWithVariousTokens(t *testing.T) {
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
			token: "very-long-token-with-many-characters-1234567890-abcdef",
		},
		{
			name:  "token with special chars",
			token: "token-with_underscores.and.dots",
		},
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "hexadecimal token",
			token: "a1b2c3d4e5f6g7h8i9j0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.token)

			if client == nil {
				t.Fatal("Expected client to be created, got nil")
			}

			if client.client == nil {
				t.Fatal("Expected linodego client to be initialized, got nil")
			}

			if client.Name() != "linode" {
				t.Errorf("Expected name 'linode', got %q", client.Name())
			}

			if client.DisplayName() != "Linode" {
				t.Errorf("Expected display name 'Linode', got %q", client.DisplayName())
			}

			if !client.SupportsProvisioning() {
				t.Error("Expected to support provisioning")
			}

			if !client.SupportsBYOS() {
				t.Error("Expected to support BYOS")
			}

			if !client.SupportsSSH() {
				t.Error("Expected to support SSH")
			}
		})
	}
}

func TestValidateCredentials_EmptyToken(t *testing.T) {
	client := NewClient("")

	if client == nil {
		t.Fatal("Expected client to be created even with empty token")
	}
}

func TestConvertInstanceToServer_NilSpecs(t *testing.T) {
	createdTime := time.Now()
	mockInstance := &linodego.Instance{
		ID:         12345,
		Label:      "test-instance",
		Status:     linodego.InstanceRunning,
		Type:       "g6-nanode-1",
		Region:     "us-east",
		Image:      "linode/ubuntu22.04",
		IPv4:       []*net.IP{parseIP("192.168.1.100")},
		Created:    &createdTime,
		Specs:      nil,
		Hypervisor: "kvm",
	}

	result := convertInstanceToServer(mockInstance)

	if result == nil {
		t.Fatal("Expected non-nil server even with nil specs")
	}

	if result.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}
}

func TestProviderErrorStructure(t *testing.T) {
	provErr := &providers.ProviderError{
		Provider: "linode",
		Code:     "test_error",
		Message:  "Test error message",
		Details:  map[string]interface{}{"key": "value"},
	}

	if provErr.Provider != "linode" {
		t.Errorf("Expected provider 'linode', got %s", provErr.Provider)
	}

	if provErr.Code != "test_error" {
		t.Errorf("Expected code 'test_error', got %s", provErr.Code)
	}

	if provErr.Message != "Test error message" {
		t.Errorf("Expected specific message, got %s", provErr.Message)
	}

	if len(provErr.Details) != 1 {
		t.Errorf("Expected 1 detail, got %d", len(provErr.Details))
	}
}

func BenchmarkStringToInt(b *testing.B) {
	testString := "12345"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = stringToInt(testString)
	}
}

func BenchmarkIntToString(b *testing.B) {
	testInt := 12345
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = intToString(testInt)
	}
}

func BenchmarkConvertInstanceToServer(b *testing.B) {
	createdTime := time.Now()
	mockInstance := &linodego.Instance{
		ID:      12345,
		Label:   "test-instance",
		Status:  linodego.InstanceRunning,
		Type:    "g6-nanode-1",
		Region:  "us-east",
		Image:   "linode/ubuntu22.04",
		IPv4:    []*net.IP{parseIP("192.168.1.100")},
		Tags:    []string{"production", "web"},
		Created: &createdTime,
		Specs: &linodego.InstanceSpec{
			VCPUs:    1,
			Memory:   1024,
			Disk:     25600,
			Transfer: 1000,
		},
		Hypervisor: "kvm",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = convertInstanceToServer(mockInstance)
	}
}

func BenchmarkExtractVersionFromLabel(b *testing.B) {
	testLabel := "Ubuntu 22.04 LTS"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractVersionFromLabel(testLabel)
	}
}

func TestEncodeUserData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "hello world",
			expected: "aGVsbG8gd29ybGQ=",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name: "cloud-init yaml",
			input: `#cloud-config
users:
  - name: deploy
    sudo: ALL=(ALL) NOPASSWD:ALL`,
			expected: "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IGRlcGxveQogICAgc3VkbzogQUxMPShBTEwpIE5PUEFTU1dEOkFMTA==",
		},
		{
			name:     "special characters",
			input:    "test@#$%^&*()",
			expected: "dGVzdEAjJCVeJiooKQ==",
		},
		{
			name:     "multiline with newlines",
			input:    "line1\nline2\nline3",
			expected: "bGluZTEKbGluZTIKbGluZTM=",
		},
		{
			name:     "unicode characters",
			input:    "Hello 世界 🌍",
			expected: "SGVsbG8g5LiW55WMIPCfjI0=",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "YQ==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeUserData(tt.input)
			if result != tt.expected {
				t.Errorf("encodeUserData(%q) = %q, want %q", tt.input, result, tt.expected)
			}

			if tt.input != "" {
				decoded, err := base64.StdEncoding.DecodeString(result)
				if err != nil {
					t.Errorf("Result is not valid base64: %v", err)
				}
				if string(decoded) != tt.input {
					t.Errorf("Decoded value %q does not match input %q", string(decoded), tt.input)
				}
			}
		})
	}
}

func TestEncodeUserData_Integration(t *testing.T) {
	cloudInitConfig := `#cloud-config
packages:
  - nginx
  - docker.io

runcmd:
  - systemctl start nginx
  - systemctl enable nginx

users:
  - name: deploy
    groups: sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL`

	encoded := encodeUserData(cloudInitConfig)

	if encoded == "" {
		t.Error("Expected non-empty encoded string")
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Encoded string is not valid base64: %v", err)
	}

	if string(decoded) != cloudInitConfig {
		t.Errorf("Decoded content does not match original")
	}

	if strings.Contains(encoded, "\n") {
		t.Error("Encoded string should not contain newlines")
	}
}

func BenchmarkEncodeUserData(b *testing.B) {
	testData := `#cloud-config
packages:
  - nginx
  - docker.io`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = encodeUserData(testData)
	}
}

func TestGenerateRootPassword(t *testing.T) {
	password, err := generateRootPassword()
	if err != nil {
		t.Fatalf("generateRootPassword() failed: %v", err)
	}

	if len(password) != 32 {
		t.Errorf("Expected password length 32, got %d", len(password))
	}

	allowedChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
	for _, char := range password {
		if !strings.ContainsRune(allowedChars, char) {
			t.Errorf("Password contains disallowed character: %c", char)
		}
	}

	passwords := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pwd, err := generateRootPassword()
		if err != nil {
			t.Fatalf("generateRootPassword() failed on iteration %d: %v", i, err)
		}
		if passwords[pwd] {
			t.Errorf("Duplicate password generated: %s", pwd)
		}
		passwords[pwd] = true
	}

	if len(passwords) != 100 {
		t.Errorf("Expected 100 unique passwords, got %d", len(passwords))
	}
}

func TestGenerateRootPasswordComplexity(t *testing.T) {
	password, err := generateRootPassword()
	if err != nil {
		t.Fatalf("generateRootPassword() failed: %v", err)
	}

	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= '0' && char <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if !(hasLower || hasUpper || hasDigit || hasSpecial) {
		t.Errorf("Password lacks character diversity: %s", password)
	}
}

func BenchmarkGenerateRootPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = generateRootPassword()
	}
}
