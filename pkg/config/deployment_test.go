package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProcessDeploymentOptions(t *testing.T) {
	tests := []struct {
		name        string
		target      *TargetConfig
		envFile     string
		envVars     []string
		skipBuild   bool
		setupFunc   func(t *testing.T) string
		expected    *DeploymentOptions
		expectError bool
	}{
		{
			name: "initialize empty deploy options",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:   "",
			envVars:   []string{},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars:   map[string]string{},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "process individual env vars",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:   "",
			envVars:   []string{"KEY1=value1", "KEY2=value2", "KEY3=value3"},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
					"KEY3": "value3",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "process env file",
			target: &TargetConfig{
				Deploy: nil,
			},
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				envFile := filepath.Join(tmpDir, ".env")
				content := `DATABASE_URL=postgres://localhost:5432/db
API_KEY=abc123
DEBUG=true`
				os.WriteFile(envFile, []byte(content), 0644)
				return envFile
			},
			envVars:   []string{},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"DATABASE_URL": "postgres://localhost:5432/db",
					"API_KEY":      "abc123",
					"DEBUG":        "true",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "merge env file and individual vars",
			target: &TargetConfig{
				Deploy: nil,
			},
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				envFile := filepath.Join(tmpDir, ".env")
				content := `FILE_VAR=from_file`
				os.WriteFile(envFile, []byte(content), 0644)
				return envFile
			},
			envVars:   []string{"CLI_VAR=from_cli"},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"FILE_VAR": "from_file",
					"CLI_VAR":  "from_cli",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "cli vars override file vars",
			target: &TargetConfig{
				Deploy: nil,
			},
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				envFile := filepath.Join(tmpDir, ".env")
				content := `OVERRIDE=from_file`
				os.WriteFile(envFile, []byte(content), 0644)
				return envFile
			},
			envVars:   []string{"OVERRIDE=from_cli"},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"OVERRIDE": "from_cli",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "skip build flag set",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:   "",
			envVars:   []string{},
			skipBuild: true,
			expected: &DeploymentOptions{
				EnvVars:   map[string]string{},
				SkipBuild: true,
			},
			expectError: false,
		},
		{
			name: "preserve existing deploy options",
			target: &TargetConfig{
				Deploy: &DeploymentOptions{
					EnvVars: map[string]string{
						"EXISTING": "value",
					},
					SkipBuild: false,
				},
			},
			envFile:   "",
			envVars:   []string{"NEW=new_value"},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"EXISTING": "value",
					"NEW":      "new_value",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "invalid env var format",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:     "",
			envVars:     []string{"INVALID_NO_EQUALS"},
			skipBuild:   false,
			expected:    nil,
			expectError: true,
		},
		{
			name: "non-existent env file",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:     "/nonexistent/.env",
			envVars:     []string{},
			skipBuild:   false,
			expected:    nil,
			expectError: true,
		},
		{
			name: "env var with equals in value",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:   "",
			envVars:   []string{"EQUATION=x=y+z", "BASE64=abc123=="},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"EQUATION": "x=y+z",
					"BASE64":   "abc123==",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "empty env var value",
			target: &TargetConfig{
				Deploy: nil,
			},
			envFile:   "",
			envVars:   []string{"EMPTY="},
			skipBuild: false,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"EMPTY": "",
				},
				SkipBuild: false,
			},
			expectError: false,
		},
		{
			name: "complex real-world scenario",
			target: &TargetConfig{
				Deploy: nil,
			},
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				envFile := filepath.Join(tmpDir, ".env.production")
				content := `# Production environment
DATABASE_URL="postgresql://user:password@db.example.com:5432/prod"
REDIS_URL="redis://cache.example.com:6379"
API_BASE_URL="https://api.example.com"
LOG_LEVEL=info
ENABLE_ANALYTICS=true`
				os.WriteFile(envFile, []byte(content), 0644)
				return envFile
			},
			envVars:   []string{"LOG_LEVEL=debug", "CUSTOM_VAR=custom_value"},
			skipBuild: true,
			expected: &DeploymentOptions{
				EnvVars: map[string]string{
					"DATABASE_URL":     "postgresql://user:password@db.example.com:5432/prod",
					"REDIS_URL":        "redis://cache.example.com:6379",
					"API_BASE_URL":     "https://api.example.com",
					"LOG_LEVEL":        "debug", // Overridden by CLI
					"ENABLE_ANALYTICS": "true",
					"CUSTOM_VAR":       "custom_value",
				},
				SkipBuild: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var envFile string
			if tt.setupFunc != nil {
				envFile = tt.setupFunc(t)
			} else {
				envFile = tt.envFile
			}

			err := tt.target.ProcessDeploymentOptions(envFile, tt.envVars, tt.skipBuild)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify Deploy options were set correctly
			if tt.target.Deploy == nil {
				t.Fatal("Deploy options should not be nil")
			}

			if !reflect.DeepEqual(tt.target.Deploy.EnvVars, tt.expected.EnvVars) {
				t.Errorf("Expected EnvVars %v, got %v", tt.expected.EnvVars, tt.target.Deploy.EnvVars)
			}

			if tt.target.Deploy.SkipBuild != tt.expected.SkipBuild {
				t.Errorf("Expected SkipBuild %v, got %v", tt.expected.SkipBuild, tt.target.Deploy.SkipBuild)
			}
		})
	}
}

func TestProcessDeploymentOptionsIdempotent(t *testing.T) {
	target := &TargetConfig{
		Deploy: nil,
	}

	envVars := []string{"KEY=value"}

	// Call twice with same options
	err1 := target.ProcessDeploymentOptions("", envVars, false)
	if err1 != nil {
		t.Fatalf("First call failed: %v", err1)
	}

	err2 := target.ProcessDeploymentOptions("", envVars, false)
	if err2 != nil {
		t.Fatalf("Second call failed: %v", err2)
	}

	// Both calls should produce same result
	if len(target.Deploy.EnvVars) != 1 || target.Deploy.EnvVars["KEY"] != "value" {
		t.Error("Multiple calls should be idempotent")
	}
}

func TestProcessDeploymentOptionsMultipleCalls(t *testing.T) {
	target := &TargetConfig{
		Deploy: nil,
	}

	// First call with some env vars
	err := target.ProcessDeploymentOptions("", []string{"VAR1=value1"}, false)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call with different env vars
	err = target.ProcessDeploymentOptions("", []string{"VAR2=value2"}, true)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	// Both vars should be present
	expected := map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	}

	if !reflect.DeepEqual(target.Deploy.EnvVars, expected) {
		t.Errorf("Expected %v, got %v", expected, target.Deploy.EnvVars)
	}

	// Skip build should be from last call
	if !target.Deploy.SkipBuild {
		t.Error("SkipBuild should be true from second call")
	}
}

func TestProcessDeploymentOptionsEnvFileWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `# This is a comment
KEY1=value1
# Another comment
KEY2=value2

# Empty line above
KEY3=value3`

	os.WriteFile(envFile, []byte(content), 0644)

	target := &TargetConfig{Deploy: nil}
	err := target.ProcessDeploymentOptions(envFile, []string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}

	if !reflect.DeepEqual(target.Deploy.EnvVars, expected) {
		t.Errorf("Expected %v, got %v", expected, target.Deploy.EnvVars)
	}
}

func TestProcessDeploymentOptionsEnvFileWithQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `DOUBLE_QUOTED="value with spaces"
SINGLE_QUOTED='another value'
NO_QUOTES=plain_value`

	os.WriteFile(envFile, []byte(content), 0644)

	target := &TargetConfig{Deploy: nil}
	err := target.ProcessDeploymentOptions(envFile, []string{}, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := map[string]string{
		"DOUBLE_QUOTED": "value with spaces",
		"SINGLE_QUOTED": "another value",
		"NO_QUOTES":     "plain_value",
	}

	if !reflect.DeepEqual(target.Deploy.EnvVars, expected) {
		t.Errorf("Expected %v, got %v", expected, target.Deploy.EnvVars)
	}
}
