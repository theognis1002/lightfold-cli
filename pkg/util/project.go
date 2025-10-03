package util

import (
	"fmt"
	"os"
	"path/filepath"
)

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
