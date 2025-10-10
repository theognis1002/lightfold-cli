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
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	sshPublicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	sshPublicKeyString := strings.TrimSpace(string(sshPublicKeyBytes))

	fingerprint := ssh.FingerprintSHA256(sshPublicKey)

	keysDir, err := GetKeysDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys directory: %w", err)
	}

	privateKeyPath := filepath.Join(keysDir, keyName)
	publicKeyPath := privateKeyPath + ".pub"

	if err := os.WriteFile(privateKeyPath, privateKeyPEM, config.PermPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to write private key: %w", err)
	}

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
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", keyPath)
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block != nil {
		_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("invalid private key format: %w", err)
			}
		}
		return nil
	}

	_, _, _, _, err = ssh.ParseAuthorizedKey(data)
	if err != nil {
		return fmt.Errorf("invalid SSH key format: %w", err)
	}

	return nil
}

// GetKeyName generates a key name for a project
func GetKeyName(projectName string) string {
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

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// DeleteKeyPair deletes an SSH key pair
func DeleteKeyPair(keyPath string) error {
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete private key: %w", err)
	}

	publicKeyPath := keyPath + ".pub"
	if err := os.Remove(publicKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete public key: %w", err)
	}

	return nil
}

// GetSSHKeyFromPath extracts the base key name from a full path
// Example: "/home/user/.lightfold/keys/lightfold_my-project_ed25519" -> "lightfold_my-project_ed25519"
func GetSSHKeyFromPath(keyPath string) string {
	return filepath.Base(keyPath)
}

// CleanupUnusedKeys removes SSH keys that are not used by any targets.
// Returns the number of keys deleted.
func CleanupUnusedKeys(allTargets map[string]config.TargetConfig) (int, error) {
	keysDir, err := GetKeysDirectory()
	if err != nil {
		return 0, fmt.Errorf("failed to get keys directory: %w", err)
	}

	keysInUse := make(map[string]bool)

	for _, target := range allTargets {
		providerCfg, err := target.GetSSHProviderConfig()
		if err != nil || providerCfg == nil {
			continue
		}

		sshKey := providerCfg.GetSSHKey()
		if sshKey != "" {
			keyName := filepath.Base(sshKey)
			keysInUse[keyName] = true
		}
	}

	entries, err := os.ReadDir(keysDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read keys directory: %w", err)
	}

	var keysToDelete []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Skip public keys - we'll delete them with their private keys
		if strings.HasSuffix(filename, ".pub") {
			continue
		}

		if !strings.HasPrefix(filename, "lightfold_") || !strings.HasSuffix(filename, "_ed25519") {
			continue
		}

		if !keysInUse[filename] {
			keysToDelete = append(keysToDelete, filepath.Join(keysDir, filename))
		}
	}

	for _, keyPath := range keysToDelete {
		if err := DeleteKeyPair(keyPath); err != nil {
			return len(keysToDelete), fmt.Errorf("failed to delete key %s: %w", filepath.Base(keyPath), err)
		}
	}

	return len(keysToDelete), nil
}
