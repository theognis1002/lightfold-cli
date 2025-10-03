package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SanitizeHostname removes invalid characters from hostname (only allows a-z, A-Z, 0-9, . and -)
// This ensures compatibility with cloud provider naming requirements
func SanitizeHostname(name string) string {
	// Replace underscores with hyphens
	name = strings.ReplaceAll(name, "_", "-")

	// Remove any character that's not alphanumeric, dot, or hyphen
	reg := regexp.MustCompile(`[^a-zA-Z0-9.-]`)
	name = reg.ReplaceAllString(name, "")

	// Remove leading/trailing dots and hyphens
	name = strings.Trim(name, ".-")

	// If empty after sanitization, use a default
	if name == "" {
		name = "app"
	}

	return name
}

// GetTargetName extracts and sanitizes a target name from a project path
// Returns the sanitized base directory name (e.g., "/path/to/my_app" -> "my-app")
func GetTargetName(projectPath string) string {
	// Clean the path first
	cleaned := filepath.Clean(projectPath)
	// Extract base name
	baseName := filepath.Base(cleaned)
	// Sanitize for use as hostname/target name
	return SanitizeHostname(baseName)
}

// ValidateProjectPath validates and cleans a project path
// Returns the cleaned absolute path or an error
func ValidateProjectPath(projectPath string) (string, error) {
	// Clean the path
	projectPath = filepath.Clean(projectPath)

	// Check if path exists
	info, err := os.Stat(projectPath)
	if err != nil {
		return "", fmt.Errorf("cannot access path '%s': %w", projectPath, err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("path '%s' is not a directory", projectPath)
	}

	// Return absolute path
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return projectPath, nil // Return cleaned path if we can't get absolute
	}

	return absPath, nil
}
