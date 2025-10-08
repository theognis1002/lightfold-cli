package ssl_test

import (
	"lightfold/pkg/ssl"
	_ "lightfold/pkg/ssl/certbot"
	"testing"
)

func TestCertbotManagerRegistration(t *testing.T) {
	manager, err := ssl.GetManager("certbot")
	if err != nil {
		t.Fatalf("Failed to get certbot manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil certbot manager")
	}

	if manager.Name() != "certbot" {
		t.Errorf("Expected name 'certbot', got '%s'", manager.Name())
	}
}

func TestCertbotManagerAvailability(t *testing.T) {
	manager, err := ssl.GetManager("certbot")
	if err != nil {
		t.Fatalf("Failed to get certbot manager: %v", err)
	}

	// Note: This will return an error because we don't have an SSH executor set
	// This is expected behavior - we're just testing the method exists
	_, err = manager.IsAvailable()
	if err == nil {
		t.Error("Expected error when SSH executor is not set")
	}
}

func TestSSLManagerList(t *testing.T) {
	managers := ssl.List()
	if len(managers) == 0 {
		t.Fatal("Expected at least one SSL manager registered")
	}

	found := false
	for _, name := range managers {
		if name == "certbot" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected certbot to be in the list of registered SSL managers")
	}
}

func TestGetNonExistentManager(t *testing.T) {
	_, err := ssl.GetManager("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent SSL manager")
	}
}
