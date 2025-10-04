package util

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    map[string]string
		expectError bool
	}{
		{
			name: "simple key-value pairs",
			content: `KEY1=value1
KEY2=value2
KEY3=value3`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
			expectError: false,
		},
		{
			name: "values with double quotes",
			content: `DATABASE_URL="postgres://user:pass@localhost:5432/db"
API_KEY="abc123def456"`,
			expected: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
				"API_KEY":      "abc123def456",
			},
			expectError: false,
		},
		{
			name: "values with single quotes",
			content: `APP_NAME='My Application'
ENV='production'`,
			expected: map[string]string{
				"APP_NAME": "My Application",
				"ENV":      "production",
			},
			expectError: false,
		},
		{
			name: "values with spaces",
			content: `MESSAGE=Hello World
PATH=/usr/local/bin:/usr/bin`,
			expected: map[string]string{
				"MESSAGE": "Hello World",
				"PATH":    "/usr/local/bin:/usr/bin",
			},
			expectError: false,
		},
		{
			name: "empty lines and comments",
			content: `# This is a comment
KEY1=value1

# Another comment
KEY2=value2

`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expectError: false,
		},
		{
			name: "comment lines with various formats",
			content: `# Comment 1
# Comment 2
KEY=value
#Inline comment without space
ANOTHER_KEY=another_value`,
			expected: map[string]string{
				"KEY":         "value",
				"ANOTHER_KEY": "another_value",
			},
			expectError: false,
		},
		{
			name: "empty values",
			content: `EMPTY1=
EMPTY2=""
EMPTY3=''`,
			expected: map[string]string{
				"EMPTY1": "",
				"EMPTY2": "",
				"EMPTY3": "",
			},
			expectError: false,
		},
		{
			name: "values with equals signs",
			content: `EQUATION=x=y+z
BASE64=SGVsbG8gV29ybGQ=`,
			expected: map[string]string{
				"EQUATION": "x=y+z",
				"BASE64":   "SGVsbG8gV29ybGQ=",
			},
			expectError: false,
		},
		{
			name: "whitespace handling",
			content: `  KEY1=value1
	KEY2=value2`,
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			expectError: false,
		},
		{
			name:        "invalid format - missing equals",
			content:     `INVALID_LINE`,
			expected:    nil,
			expectError: true,
		},
		{
			name: "complex values with special characters",
			content: `JWT_SECRET="my-super-secret-key-!@#$%^&*()"
DATABASE_URL='postgresql://user:p@ssw0rd@localhost/db'`,
			expected: map[string]string{
				"JWT_SECRET":   "my-super-secret-key-!@#$%^&*()",
				"DATABASE_URL": "postgresql://user:p@ssw0rd@localhost/db",
			},
			expectError: false,
		},
		{
			name: "mixed quotes and no quotes",
			content: `PLAIN=value
SINGLE='value'
DOUBLE="value"`,
			expected: map[string]string{
				"PLAIN":  "value",
				"SINGLE": "value",
				"DOUBLE": "value",
			},
			expectError: false,
		},
		{
			name:        "empty file",
			content:     "",
			expected:    map[string]string{},
			expectError: false,
		},
		{
			name: "only comments",
			content: `# Comment 1
# Comment 2
# Comment 3`,
			expected:    map[string]string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, ".env")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			// Test LoadEnvFile
			result, err := LoadEnvFile(tmpFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadEnvFileNonExistent(t *testing.T) {
	_, err := LoadEnvFile("/nonexistent/path/.env")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSplitEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple key-value",
			input:    "KEY=value",
			expected: []string{"KEY", "value"},
		},
		{
			name:     "value with equals",
			input:    "EQUATION=x=y+z",
			expected: []string{"EQUATION", "x=y+z"},
		},
		{
			name:     "empty value",
			input:    "EMPTY=",
			expected: []string{"EMPTY", ""},
		},
		{
			name:     "no equals sign",
			input:    "NOEQUALS",
			expected: []string{"NOEQUALS"},
		},
		{
			name:     "multiple equals",
			input:    "KEY=value=more=data",
			expected: []string{"KEY", "value=more=data"},
		},
		{
			name:     "equals at start",
			input:    "=value",
			expected: []string{"", "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitEnvVar(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single line",
			input:    "line1",
			expected: []string{"line1"},
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "empty lines",
			input:    "line1\n\nline3",
			expected: []string{"line1", "", "line3"},
		},
		{
			name:     "trailing newline",
			input:    "line1\nline2\n",
			expected: []string{"line1", "line2"},
		},
		{
			name:     "only newlines",
			input:    "\n\n\n",
			expected: []string{"", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitLines(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no whitespace",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "leading space",
			input:    "  hello",
			expected: "hello",
		},
		{
			name:     "trailing space",
			input:    "hello  ",
			expected: "hello",
		},
		{
			name:     "both sides",
			input:    "  hello  ",
			expected: "hello",
		},
		{
			name:     "tabs",
			input:    "\t\thello\t\t",
			expected: "hello",
		},
		{
			name:     "newlines",
			input:    "\n\nhello\n\n",
			expected: "hello",
		},
		{
			name:     "carriage returns",
			input:    "\r\rhello\r\r",
			expected: "hello",
		},
		{
			name:     "mixed whitespace",
			input:    " \t\n\rhello \t\n\r",
			expected: "hello",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n\r   ",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace in middle",
			input:    "hello world",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimSpace(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestStartsWithHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "starts with hash",
			input:    "# This is a comment",
			expected: true,
		},
		{
			name:     "hash without space",
			input:    "#comment",
			expected: true,
		},
		{
			name:     "does not start with hash",
			input:    "KEY=value",
			expected: false,
		},
		{
			name:     "hash in middle",
			input:    "KEY=value # comment",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "only hash",
			input:    "#",
			expected: true,
		},
		{
			name:     "whitespace before hash",
			input:    "  # comment",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartsWithHash(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadEnvFileWithRealWorldExample(t *testing.T) {
	content := `# Database Configuration
DATABASE_URL="postgresql://user:password@localhost:5432/mydb"
DATABASE_POOL_SIZE=10

# API Keys
API_KEY=abc123def456
SECRET_KEY="my-secret-key-!@#$%"

# Feature Flags
ENABLE_FEATURE_X=true
ENABLE_FEATURE_Y=false

# Empty values
OPTIONAL_CONFIG=

# Comments and empty lines
# This is ignored

DEBUG_MODE=true
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	result, err := LoadEnvFile(tmpFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := map[string]string{
		"DATABASE_URL":       "postgresql://user:password@localhost:5432/mydb",
		"DATABASE_POOL_SIZE": "10",
		"API_KEY":            "abc123def456",
		"SECRET_KEY":         "my-secret-key-!@#$%",
		"ENABLE_FEATURE_X":   "true",
		"ENABLE_FEATURE_Y":   "false",
		"OPTIONAL_CONFIG":    "",
		"DEBUG_MODE":         "true",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}
