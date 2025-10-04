package deploy

import (
	"lightfold/pkg/deploy"
	"lightfold/pkg/detector"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestParseEnvFile tests the parseEnvFile function
func TestParseEnvFile(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name: "basic key-value pairs",
			content: `DATABASE_URL=postgres://localhost:5432/mydb
API_KEY=secret123
PORT=3000`,
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/mydb",
				"API_KEY":      "secret123",
				"PORT":         "3000",
			},
		},
		{
			name: "quoted values",
			content: `NAME="My App"
SECRET='super-secret'
PATH="/usr/bin:/bin"`,
			expected: map[string]string{
				"NAME":   "My App",
				"SECRET": "super-secret",
				"PATH":   "/usr/bin:/bin",
			},
		},
		{
			name: "comments and empty lines",
			content: `# Database configuration
DATABASE_URL=postgres://localhost:5432/mydb

# API Settings
API_KEY=secret123
# PORT=8080
REAL_PORT=3000`,
			expected: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/mydb",
				"API_KEY":      "secret123",
				"REAL_PORT":    "3000",
			},
		},
		{
			name: "values with equals signs",
			content: `JWT_SECRET=abc123=def456=ghi789
EQUATION=2+2=4`,
			expected: map[string]string{
				"JWT_SECRET": "abc123=def456=ghi789",
				"EQUATION":   "2+2=4",
			},
		},
		{
			name: "empty values",
			content: `EMPTY=
BLANK=""
SINGLE=''`,
			expected: map[string]string{
				"EMPTY":  "",
				"BLANK":  "",
				"SINGLE": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file
			testFile := filepath.Join(tmpDir, "test.env")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Parse the file
			result := deploy.ParseEnvFile(testFile)

			// Compare results
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseEnvFile() = %v, want %v", result, tt.expected)
			}

			// Cleanup
			os.Remove(testFile)
		})
	}
}

// TestFindEnvFile tests the env file priority logic
func TestFindEnvFile(t *testing.T) {
	tests := []struct {
		name          string
		filesToCreate []string
		expectedFile  string
	}{
		{
			name:          "finds .env.production first",
			filesToCreate: []string{".env", ".env.prod", ".env.production"},
			expectedFile:  ".env.production",
		},
		{
			name:          "falls back to .env.prod",
			filesToCreate: []string{".env", ".env.prod"},
			expectedFile:  ".env.prod",
		},
		{
			name:          "falls back to .env",
			filesToCreate: []string{".env"},
			expectedFile:  ".env",
		},
		{
			name:          "returns empty when no files exist",
			filesToCreate: []string{},
			expectedFile:  "",
		},
		{
			name:          "ignores .env.local and other variants",
			filesToCreate: []string{".env.local", ".env.development"},
			expectedFile:  "",
		},
		{
			name:          "prefers .env.production over .env even if .env exists",
			filesToCreate: []string{".env"},
			expectedFile:  ".env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary test directory
			tmpDir := t.TempDir()

			// Create test files
			for _, file := range tt.filesToCreate {
				path := filepath.Join(tmpDir, file)
				if err := os.WriteFile(path, []byte("TEST=value"), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", file, err)
				}
			}

			// Create executor with test directory
			detection := &detector.Detection{
				Framework: "Next.js",
				Language:  "JavaScript/TypeScript",
			}
			executor := deploy.NewExecutor(nil, "test-app", tmpDir, detection)

			// Find env file
			result := executor.FindEnvFile()

			// Get just the filename for comparison
			var resultFile string
			if result != "" {
				resultFile = filepath.Base(result)
			}

			if resultFile != tt.expectedFile {
				t.Errorf("findEnvFile() = %s, want %s", resultFile, tt.expectedFile)
			}
		})
	}
}

// TestLoadLocalEnvFile tests the complete env loading logic
func TestLoadLocalEnvFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .env.production
	envProduction := filepath.Join(tmpDir, ".env.production")
	productionContent := `DATABASE_URL=postgres://prod-db:5432/mydb
API_KEY=prod-secret
NEXT_PUBLIC_API_URL=https://api.example.com`
	if err := os.WriteFile(envProduction, []byte(productionContent), 0644); err != nil {
		t.Fatalf("Failed to create .env.production: %v", err)
	}

	// Create .env as fallback
	envDefault := filepath.Join(tmpDir, ".env")
	defaultContent := `DATABASE_URL=postgres://localhost:5432/mydb
API_KEY=dev-secret`
	if err := os.WriteFile(envDefault, []byte(defaultContent), 0644); err != nil {
		t.Fatalf("Failed to create .env: %v", err)
	}

	// Create executor
	detection := &detector.Detection{
		Framework: "Next.js",
		Language:  "JavaScript/TypeScript",
	}
	executor := deploy.NewExecutor(nil, "test-app", tmpDir, detection)

	// Load env file - should load .env.production, not .env
	result := executor.LoadLocalEnvFile()

	expected := map[string]string{
		"DATABASE_URL":        "postgres://prod-db:5432/mydb",
		"API_KEY":             "prod-secret",
		"NEXT_PUBLIC_API_URL": "https://api.example.com",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("loadLocalEnvFile() = %v, want %v", result, expected)
	}
}
