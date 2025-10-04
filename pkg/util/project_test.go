package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeHostname(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "alphanumeric only",
			input:    "myapp",
			expected: "myapp",
		},
		{
			name:     "underscores to hyphens",
			input:    "my_app_name",
			expected: "my-app-name",
		},
		{
			name:     "special characters removed",
			input:    "my@app#name$",
			expected: "myappname",
		},
		{
			name:     "dots preserved",
			input:    "app.example.com",
			expected: "app.example.com",
		},
		{
			name:     "hyphens preserved",
			input:    "my-app-name",
			expected: "my-app-name",
		},
		{
			name:     "leading/trailing dots removed",
			input:    ".myapp.",
			expected: "myapp",
		},
		{
			name:     "leading/trailing hyphens removed",
			input:    "-myapp-",
			expected: "myapp",
		},
		{
			name:     "mixed special chars",
			input:    "my_app!@#$%^&*()name",
			expected: "my-appname",
		},
		{
			name:     "spaces removed",
			input:    "my app name",
			expected: "myappname",
		},
		{
			name:     "empty after sanitization",
			input:    "!@#$%^&*()",
			expected: "app",
		},
		{
			name:     "only underscores",
			input:    "___",
			expected: "app",
		},
		{
			name:     "unicode characters removed",
			input:    "myapp™©®",
			expected: "myapp",
		},
		{
			name:     "numbers preserved",
			input:    "app123",
			expected: "app123",
		},
		{
			name:     "mixed case preserved",
			input:    "MyAppName",
			expected: "MyAppName",
		},
		{
			name:     "complex real-world example",
			input:    "my_web_app-v2.0",
			expected: "my-web-app-v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHostname(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetTargetName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "/path/to/myapp",
			expected: "myapp",
		},
		{
			name:     "path with underscores",
			input:    "/path/to/my_app",
			expected: "my-app",
		},
		{
			name:     "relative path",
			input:    "./my_project",
			expected: "my-project",
		},
		{
			name:     "current directory",
			input:    ".",
			expected: "app", // Sanitizer converts empty to "app"
		},
		{
			name:     "home directory",
			input:    "~/projects/my_app",
			expected: "my-app",
		},
		{
			name:     "trailing slash",
			input:    "/path/to/myapp/",
			expected: "myapp",
		},
		{
			name:     "multiple trailing slashes",
			input:    "/path/to/myapp///",
			expected: "myapp",
		},
		{
			name:     "path with special chars",
			input:    "/path/to/my@app#name",
			expected: "myappname",
		},
		// Note: Windows paths don't work correctly on Unix (filepath.Base behavior differs)
		// Skipping windows path test on Unix systems
		{
			name:     "path with spaces",
			input:    "/path/to/my app",
			expected: "myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTargetName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestValidateProjectPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setupFunc   func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid directory",
			setupFunc: func() string {
				dir := filepath.Join(tmpDir, "valid_dir")
				os.Mkdir(dir, 0755)
				return dir
			},
			expectError: false,
		},
		{
			name: "non-existent path",
			setupFunc: func() string {
				return filepath.Join(tmpDir, "nonexistent")
			},
			expectError: true,
			errorMsg:    "cannot access path",
		},
		{
			name: "file instead of directory",
			setupFunc: func() string {
				file := filepath.Join(tmpDir, "file.txt")
				os.WriteFile(file, []byte("content"), 0644)
				return file
			},
			expectError: true,
			errorMsg:    "is not a directory",
		},
		{
			name: "relative path to existing directory",
			setupFunc: func() string {
				// Create a subdirectory in tmpDir
				dir := filepath.Join(tmpDir, "relative_dir")
				os.Mkdir(dir, 0755)
				// Change to parent directory
				originalDir, _ := os.Getwd()
				os.Chdir(tmpDir)
				// Schedule cleanup to restore directory
				t.Cleanup(func() {
					os.Chdir(originalDir)
				})
				return "relative_dir"
			},
			expectError: false,
		},
		{
			name: "path with trailing slash",
			setupFunc: func() string {
				dir := filepath.Join(tmpDir, "trailing_slash")
				os.Mkdir(dir, 0755)
				return dir + "/"
			},
			expectError: false,
		},
		{
			name: "nested directory",
			setupFunc: func() string {
				dir := filepath.Join(tmpDir, "parent", "child", "nested")
				os.MkdirAll(dir, 0755)
				return dir
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupFunc()
			result, err := ValidateProjectPath(path)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify the result is an absolute path
			if !filepath.IsAbs(result) {
				t.Errorf("Expected absolute path, got: %s", result)
			}

			// Verify the path exists and is accessible
			info, err := os.Stat(result)
			if err != nil {
				t.Errorf("Result path is not accessible: %v", err)
			}
			if !info.IsDir() {
				t.Error("Result path is not a directory")
			}
		})
	}
}

func TestValidateProjectPathCleansPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test directory
	testDir := filepath.Join(tmpDir, "test")
	os.Mkdir(testDir, 0755)

	// Test with messy path
	messyPath := filepath.Join(tmpDir, "test", "..", "test", ".", ".")
	result, err := ValidateProjectPath(messyPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the path was cleaned
	expected := testDir
	if result != expected {
		t.Errorf("Expected cleaned path '%s', got '%s'", expected, result)
	}
}

func TestValidateProjectPathSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real directory
	realDir := filepath.Join(tmpDir, "real")
	os.Mkdir(realDir, 0755)

	// Create a symlink to it
	symlinkPath := filepath.Join(tmpDir, "symlink")
	err := os.Symlink(realDir, symlinkPath)
	if err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}

	// Validate the symlink path
	result, err := ValidateProjectPath(symlinkPath)
	if err != nil {
		t.Errorf("Symlink should be valid: %v", err)
	}

	// Verify it's accessible
	info, err := os.Stat(result)
	if err != nil {
		t.Errorf("Symlink result is not accessible: %v", err)
	}
	if !info.IsDir() {
		t.Error("Symlink should resolve to a directory")
	}
}

func TestSanitizeHostnameEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "very long name",
			input:    "this_is_a_very_long_application_name_that_might_cause_issues",
			expected: "this-is-a-very-long-application-name-that-might-cause-issues",
		},
		{
			name:     "consecutive special chars",
			input:    "app___name",
			expected: "app---name",
		},
		{
			name:     "dots and hyphens mixed",
			input:    ".-app.-name.-",
			expected: "app.-name",
		},
		{
			name:     "only dots",
			input:    "...",
			expected: "app",
		},
		{
			name:     "only hyphens",
			input:    "---",
			expected: "app",
		},
		{
			name:     "alphanumeric boundaries",
			input:    "app123_name456",
			expected: "app123-name456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeHostname(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
