package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"lightfold/pkg/config"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// KeyPair represents an SSH key pair
type KeyPair struct {
	PrivateKeyPath string `json:"private_key_path"`
	PublicKeyPath  string `json:"public_key_path"`
	PublicKey      string `json:"public_key"` // SSH public key content
	Fingerprint    string `json:"fingerprint"`
}

// GenerateKeyPair generates a new Ed25519 SSH key pair
func GenerateKeyPair(keyName string) (*KeyPair, error) {
	// Generate Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create SSH public key
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Convert private key to PEM format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Format SSH public key
	sshPublicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	sshPublicKeyString := strings.TrimSpace(string(sshPublicKeyBytes))

	// Get fingerprint
	fingerprint := ssh.FingerprintSHA256(sshPublicKey)

	// Get lightfold keys directory
	keysDir, err := GetKeysDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys directory: %w", err)
	}

	// Create key file paths
	privateKeyPath := filepath.Join(keysDir, keyName)
	publicKeyPath := privateKeyPath + ".pub"

	// Write private key
	if err := os.WriteFile(privateKeyPath, privateKeyPEM, config.PermPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to write private key: %w", err)
	}

	// Write public key
	if err := os.WriteFile(publicKeyPath, sshPublicKeyBytes, config.PermPublicKey); err != nil {
		return nil, fmt.Errorf("failed to write public key: %w", err)
	}

	return &KeyPair{
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		PublicKey:      sshPublicKeyString,
		Fingerprint:    fingerprint,
	}, nil
}

// GetKeysDirectory returns the lightfold SSH keys directory
func GetKeysDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	keysDir := filepath.Join(homeDir, config.LocalConfigDir, config.LocalKeysDir)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create keys directory: %w", err)
	}

	return keysDir, nil
}

// LoadPublicKey loads an SSH public key from a file
func LoadPublicKey(publicKeyPath string) (string, error) {
	data, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// ValidateSSHKey validates that an SSH key file exists and is valid
func ValidateSSHKey(keyPath string) error {
	// Check if file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", keyPath)
	}

	// Try to read and parse the key
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key file: %w", err)
	}

	// Try to parse as private key first
	block, _ := pem.Decode(data)
	if block != nil {
		// It's a PEM-encoded private key
		_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			// Try parsing as RSA private key
			_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("invalid private key format: %w", err)
			}
		}
		return nil
	}

	// Try to parse as SSH public key
	_, _, _, _, err = ssh.ParseAuthorizedKey(data)
	if err != nil {
		return fmt.Errorf("invalid SSH key format: %w", err)
	}

	return nil
}

// GetKeyName generates a key name for a project
func GetKeyName(projectName string) string {
	// Sanitize project name for file system
	keyName := strings.ReplaceAll(projectName, " ", "_")
	keyName = strings.ReplaceAll(keyName, "/", "_")
	keyName = strings.ReplaceAll(keyName, "\\", "_")
	keyName = strings.ToLower(keyName)

	return fmt.Sprintf("lightfold_%s_ed25519", keyName)
}

// KeyExists checks if a key pair already exists
func KeyExists(keyName string) (bool, error) {
	keysDir, err := GetKeysDirectory()
	if err != nil {
		return false, err
	}

	privateKeyPath := filepath.Join(keysDir, keyName)
	publicKeyPath := privateKeyPath + ".pub"

	// Check if both files exist
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}
