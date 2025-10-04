package providers

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockProvider implements the Provider interface for testing
type MockProvider struct {
	name                 string
	displayName          string
	supportsProvisioning bool
	supportsBYOS         bool
}

func (m *MockProvider) Name() string                 { return m.name }
func (m *MockProvider) DisplayName() string          { return m.displayName }
func (m *MockProvider) SupportsProvisioning() bool   { return m.supportsProvisioning }
func (m *MockProvider) SupportsBYOS() bool           { return m.supportsBYOS }
func (m *MockProvider) ValidateCredentials(ctx context.Context) error   { return nil }
func (m *MockProvider) GetRegions(ctx context.Context) ([]Region, error) { return nil, nil }
func (m *MockProvider) GetSizes(ctx context.Context, region string) ([]Size, error) { return nil, nil }
func (m *MockProvider) GetImages(ctx context.Context) ([]Image, error) { return nil, nil }
func (m *MockProvider) Provision(ctx context.Context, config ProvisionConfig) (*Server, error) {
	return nil, nil
}
func (m *MockProvider) GetServer(ctx context.Context, serverID string) (*Server, error) { return nil, nil }
func (m *MockProvider) Destroy(ctx context.Context, serverID string) error { return nil }
func (m *MockProvider) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*Server, error) {
	return nil, nil
}
func (m *MockProvider) UploadSSHKey(ctx context.Context, name, publicKey string) (*SSHKey, error) {
	return nil, nil
}

func TestRegister(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Test registering a new provider
	factory := func(token string) Provider {
		return &MockProvider{
			name:        "testprovider",
			displayName: "Test Provider",
		}
	}

	Register("testprovider", factory)

	// Verify provider was registered
	if !IsRegistered("testprovider") {
		t.Error("Expected provider to be registered")
	}
}

func TestGetProvider(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register a test provider
	factory := func(token string) Provider {
		return &MockProvider{
			name:        "mockprovider",
			displayName: "Mock Provider",
		}
	}
	Register("mockprovider", factory)

	// Test getting an existing provider
	provider, err := GetProvider("mockprovider", "test-token")
	if err != nil {
		t.Fatalf("Expected no error getting provider, got: %v", err)
	}
	if provider == nil {
		t.Fatal("Expected provider to be returned")
	}
	if provider.Name() != "mockprovider" {
		t.Errorf("Expected provider name 'mockprovider', got '%s'", provider.Name())
	}

	// Test getting a non-existent provider
	_, err = GetProvider("nonexistent", "test-token")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestListProviders(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register multiple providers
	providers := []string{"provider1", "provider2", "provider3"}
	for _, name := range providers {
		providerName := name // Capture for closure
		factory := func(token string) Provider {
			return &MockProvider{name: providerName}
		}
		Register(name, factory)
	}

	// List providers
	listed := ListProviders()
	if len(listed) != len(providers) {
		t.Errorf("Expected %d providers, got %d", len(providers), len(listed))
	}

	// Verify all providers are in the list
	for _, expected := range providers {
		found := false
		for _, name := range listed {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider '%s' in list", expected)
		}
	}
}

func TestIsRegistered(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register a provider
	factory := func(token string) Provider {
		return &MockProvider{name: "registered"}
	}
	Register("registered", factory)

	// Test registered provider
	if !IsRegistered("registered") {
		t.Error("Expected 'registered' to be registered")
	}

	// Test non-registered provider
	if IsRegistered("notregistered") {
		t.Error("Expected 'notregistered' to not be registered")
	}
}

func TestConcurrentRegistration(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Concurrently register providers
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			name := "provider" + string(rune('0'+index))
			factory := func(token string) Provider {
				return &MockProvider{name: name}
			}
			Register(name, factory)
		}(i)
	}

	wg.Wait()

	// Verify all providers were registered
	listed := ListProviders()
	if len(listed) != 10 {
		t.Errorf("Expected 10 providers after concurrent registration, got %d", len(listed))
	}
}

func TestGetProviderInfo(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register a provider with specific metadata
	factory := func(token string) Provider {
		return &MockProvider{
			name:                 "infoprovider",
			displayName:          "Info Provider",
			supportsProvisioning: true,
			supportsBYOS:         true,
		}
	}
	Register("infoprovider", factory)

	// Get provider info
	info, err := GetProviderInfo("infoprovider", "")
	if err != nil {
		t.Fatalf("Expected no error getting provider info, got: %v", err)
	}

	// Verify metadata
	if info.Name != "infoprovider" {
		t.Errorf("Expected name 'infoprovider', got '%s'", info.Name)
	}
	if info.DisplayName != "Info Provider" {
		t.Errorf("Expected display name 'Info Provider', got '%s'", info.DisplayName)
	}
	if !info.SupportsProvisioning {
		t.Error("Expected SupportsProvisioning to be true")
	}
	if !info.SupportsBYOS {
		t.Error("Expected SupportsBYOS to be true")
	}

	// Test non-existent provider
	_, err = GetProviderInfo("nonexistent", "")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestGetAllProviderInfo(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register multiple providers
	providers := []struct {
		name        string
		displayName string
	}{
		{"provider1", "Provider 1"},
		{"provider2", "Provider 2"},
		{"provider3", "Provider 3"},
	}

	for _, p := range providers {
		name := p.name
		displayName := p.displayName
		factory := func(token string) Provider {
			return &MockProvider{
				name:        name,
				displayName: displayName,
			}
		}
		Register(name, factory)
	}

	// Get all provider info
	infos, err := GetAllProviderInfo()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(infos) != len(providers) {
		t.Errorf("Expected %d provider infos, got %d", len(providers), len(infos))
	}

	// Verify all providers are in the list
	for _, expected := range providers {
		found := false
		for _, info := range infos {
			if info.Name == expected.name && info.DisplayName == expected.displayName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider '%s' in info list", expected.name)
		}
	}
}

func TestProviderFactoryWithToken(t *testing.T) {
	// Create a fresh registry for testing
	testRegistry := &Registry{
		providers: make(map[string]ProviderFactory),
	}

	// Save and restore global registry
	oldRegistry := globalRegistry
	globalRegistry = testRegistry
	defer func() { globalRegistry = oldRegistry }()

	// Register a provider that uses the token
	var capturedToken string
	factory := func(token string) Provider {
		capturedToken = token
		return &MockProvider{name: "tokenprovider"}
	}
	Register("tokenprovider", factory)

	// Get provider with token
	expectedToken := "test-token-123"
	_, err := GetProvider("tokenprovider", expectedToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if capturedToken != expectedToken {
		t.Errorf("Expected token '%s' to be passed to factory, got '%s'", expectedToken, capturedToken)
	}
}
