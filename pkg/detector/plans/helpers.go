package plans

import (
	"os"
	"path/filepath"
)

// fileExists checks if a file exists at the given path
func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

// dirExists checks if a directory exists at the given path
func dirExists(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, rel))
	return err == nil && fi.IsDir()
}

// readFile reads a file and returns its content as a string
func readFile(root, rel string) string {
	b, _ := os.ReadFile(filepath.Join(root, rel))
	return string(b)
}
