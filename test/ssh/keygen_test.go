package ssh

import (
	"lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyGeneration(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	// Override the keys directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	keyName := "test_key_ed25519"
	keyPair, err := ssh.GenerateKeyPair(keyName)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Verify key pair structure
	if keyPair.PrivateKeyPath == "" {
		t.Error("Private key path should not be empty")
	}

	if keyPair.PublicKeyPath == "" {
		t.Error("Public key path should not be empty")
	}

	if keyPair.PublicKey == "" {
		t.Error("Public key content should not be empty")
	}

	if keyPair.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}

	// Verify files exist
	if _, err := os.Stat(keyPair.PrivateKeyPath); os.IsNotExist(err) {
		t.Error("Private key file should exist")
	}

	if _, err := os.Stat(keyPair.PublicKeyPath); os.IsNotExist(err) {
		t.Error("Public key file should exist")
	}

	// Verify public key format
	if !strings.Contains(keyPair.PublicKey, "ssh-ed25519") {
		t.Error("Public key should be in Ed25519 format")
	}

	// Verify fingerprint format
	if !strings.HasPrefix(keyPair.Fingerprint, "SHA256:") {
		t.Error("Fingerprint should start with SHA256:")
	}
}

func TestKeysDirectory(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	keysDir, err := ssh.GetKeysDirectory()
	if err != nil {
		t.Fatalf("Failed to get keys directory: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".lightfold", "keys")
	if keysDir != expectedPath {
		t.Errorf("Expected keys directory %s, got %s", expectedPath, keysDir)
	}

	// Verify directory was created
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		t.Error("Keys directory should be created")
	}

	// Verify directory permissions
	info, err := os.Stat(keysDir)
	if err != nil {
		t.Fatalf("Failed to stat keys directory: %v", err)
	}

	if info.Mode().Perm() != 0700 {
		t.Errorf("Expected directory permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestKeyNameGeneration(t *testing.T) {
	testCases := []struct {
		projectName string
		expected    string
	}{
		{
			projectName: "my-app",
			expected:    "lightfold_my-app_ed25519",
		},
		{
			projectName: "my app with spaces",
			expected:    "lightfold_my_app_with_spaces_ed25519",
		},
		{
			projectName: "app/with/slashes",
			expected:    "lightfold_app_with_slashes_ed25519",
		},
		{
			projectName: "app\\with\\backslashes",
			expected:    "lightfold_app_with_backslashes_ed25519",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.projectName, func(t *testing.T) {
			keyName := ssh.GetKeyName(tc.projectName)
			if keyName != tc.expected {
				t.Errorf("Expected key name %s, got %s", tc.expected, keyName)
			}
		})
	}
}

func TestKeyValidation(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name    string
		keyPath string
		content string
		valid   bool
	}{
		{
			name:    "valid Ed25519 private key",
			keyPath: filepath.Join(tempDir, "ed25519_key"),
			content: `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIJ+DYvh6SEqVTm50DFtMDoQikTmiCqirVv9mWG9qfSnF
-----END PRIVATE KEY-----`,
			valid: true,
		},
		{
			name:    "valid Ed25519 public key",
			keyPath: filepath.Join(tempDir, "ed25519_pub_key"),
			content: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOM/5/Y5wMRJhP3UAjiVyxuHgqKvvMV9p3QS8jU8bwYB test@example.com",
			valid:   true,
		},
		{
			name:    "invalid key content",
			keyPath: filepath.Join(tempDir, "invalid_key"),
			content: "this is not a valid key",
			valid:   false,
		},
		{
			name:    "empty file",
			keyPath: filepath.Join(tempDir, "empty_key"),
			content: "",
			valid:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write test file
			err := os.WriteFile(tc.keyPath, []byte(tc.content), 0600)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			err = ssh.ValidateSSHKey(tc.keyPath)
			if tc.valid && err != nil {
				t.Errorf("Expected valid key, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("Expected invalid key, got no error")
			}
		})
	}
}

func TestKeyExists(t *testing.T) {
	tempDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	keyName := "test_existing_key"

	// Check non-existent key
	exists, err := ssh.KeyExists(keyName)
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}
	if exists {
		t.Error("Key should not exist initially")
	}

	// Generate key
	_, err = ssh.GenerateKeyPair(keyName)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Check existing key
	exists, err = ssh.KeyExists(keyName)
	if err != nil {
		t.Fatalf("Failed to check key existence: %v", err)
	}
	if !exists {
		t.Error("Key should exist after generation")
	}
}

func TestLoadPublicKey(t *testing.T) {
	tempDir := t.TempDir()

	publicKeyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGq1234567890abcdef test@example.com"
	publicKeyPath := filepath.Join(tempDir, "test_key.pub")

	// Write test public key
	err := os.WriteFile(publicKeyPath, []byte(publicKeyContent+"\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test public key: %v", err)
	}

	// Load public key
	loadedKey, err := ssh.LoadPublicKey(publicKeyPath)
	if err != nil {
		t.Fatalf("Failed to load public key: %v", err)
	}

	if loadedKey != publicKeyContent {
		t.Errorf("Expected public key content %s, got %s", publicKeyContent, loadedKey)
	}

	// Test non-existent file
	_, err = ssh.LoadPublicKey(filepath.Join(tempDir, "nonexistent.pub"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
