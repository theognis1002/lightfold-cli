package deploy

import (
	"lightfold/pkg/util"
	"path/filepath"
	"testing"
)

func TestSanitizeHostname(t *testing.T) {
	// Note: This test now uses util.SanitizeHostname instead of the local function
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "underscores to hyphens",
			input: "my_project_name",
			want:  "my-project-name",
		},
		{
			name:  "remove special characters",
			input: "my@project!name",
			want:  "myprojectname",
		},
		{
			name:  "spaces removed",
			input: "my project name",
			want:  "myprojectname",
		},
		{
			name:  "mixed invalid characters",
			input: "my_project@2024!v1",
			want:  "my-project2024v1",
		},
		{
			name:  "leading and trailing dots removed",
			input: ".my-project.",
			want:  "my-project",
		},
		{
			name:  "leading and trailing hyphens removed",
			input: "-my-project-",
			want:  "my-project",
		},
		{
			name:  "valid hostname unchanged",
			input: "my-project",
			want:  "my-project",
		},
		{
			name:  "alphanumeric only",
			input: "project123",
			want:  "project123",
		},
		{
			name:  "dots preserved in middle",
			input: "my.project.name",
			want:  "my.project.name",
		},
		{
			name:  "empty string becomes app",
			input: "",
			want:  "app",
		},
		{
			name:  "only special chars becomes app",
			input: "@#$%^&*",
			want:  "app",
		},
		{
			name:  "unicode characters removed",
			input: "my-project-™",
			want:  "my-project",
		},
		{
			name:  "camelCase preserved",
			input: "myProjectName",
			want:  "myProjectName",
		},
		{
			name:  "mixed case with underscores",
			input: "My_Project_Name",
			want:  "My-Project-Name",
		},
		{
			name:  "numbers preserved",
			input: "project-2024-v1.5",
			want:  "project-2024-v1.5",
		},
		{
			name:  "complex real-world example",
			input: "my_awesome_app@v2.0_beta!",
			want:  "my-awesome-appv2.0-beta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.SanitizeHostname(tt.input)
			if got != tt.want {
				t.Errorf("util.SanitizeHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeHostname_DigitalOceanCompliance(t *testing.T) {
	// DigitalOcean only allows: a-z, A-Z, 0-9, . and -
	tests := []struct {
		name  string
		input string
	}{
		{name: "underscores", input: "test_project"},
		{name: "spaces", input: "test project"},
		{name: "special chars", input: "test@project!"},
		{name: "unicode", input: "test™project"},
		{name: "parentheses", input: "test(project)"},
		{name: "brackets", input: "test[project]"},
		{name: "slashes", input: "test/project"},
		{name: "backslashes", input: "test\\project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.SanitizeHostname(tt.input)

			// Verify only valid characters remain
			for _, char := range result {
				isValid := (char >= 'a' && char <= 'z') ||
					(char >= 'A' && char <= 'Z') ||
					(char >= '0' && char <= '9') ||
					char == '.' ||
					char == '-'

				if !isValid {
					t.Errorf("util.SanitizeHostname(%q) = %q contains invalid character %q", tt.input, result, char)
				}
			}

			// Verify no leading/trailing dots or hyphens
			if len(result) > 0 {
				firstChar := result[0]
				lastChar := result[len(result)-1]

				if firstChar == '.' || firstChar == '-' {
					t.Errorf("util.SanitizeHostname(%q) = %q starts with invalid character %q", tt.input, result, firstChar)
				}
				if lastChar == '.' || lastChar == '-' {
					t.Errorf("util.SanitizeHostname(%q) = %q ends with invalid character %q", tt.input, result, lastChar)
				}
			}
		})
	}
}

func TestSanitizeHostname_DropletNameFormat(t *testing.T) {
	// Test the actual format that will be used for droplet names
	projectNames := []string{
		"my_web_app",
		"api-service",
		"frontend@2024",
		"backend_v2.0",
		"mobile-app!prod",
	}

	for _, projectName := range projectNames {
		t.Run(projectName, func(t *testing.T) {
			sanitized := util.SanitizeHostname(projectName)
			dropletName := sanitized + "-app"

			// Verify droplet name is valid
			for _, char := range dropletName {
				isValid := (char >= 'a' && char <= 'z') ||
					(char >= 'A' && char <= 'Z') ||
					(char >= '0' && char <= '9') ||
					char == '.' ||
					char == '-'

				if !isValid {
					t.Errorf("Droplet name %q contains invalid character %q (from project %q)", dropletName, char, projectName)
				}
			}

			// Verify it's not empty
			if dropletName == "" || dropletName == "-app" {
				t.Errorf("Droplet name %q is invalid for project %q", dropletName, projectName)
			}

			t.Logf("Project %q → Sanitized %q → Droplet %q", projectName, sanitized, dropletName)
		})
	}
}

func TestSanitizeHostname_WithProjectPath(t *testing.T) {
	tests := []struct {
		name        string
		projectPath string
		wantBase    string
		wantDroplet string
	}{
		{
			name:        "absolute path",
			projectPath: "/Users/john/Documents/Projects/my-api",
			wantBase:    "my-api",
			wantDroplet: "my-api-app",
		},
		{
			name:        "path with underscores",
			projectPath: "/Users/john/Documents/Projects/dummy_api",
			wantBase:    "dummy_api",
			wantDroplet: "dummy-api-app",
		},
		{
			name:        "long absolute path",
			projectPath: "/Users/michaelmcclelland/Documents/Projects/dummy-api",
			wantBase:    "dummy-api",
			wantDroplet: "dummy-api-app",
		},
		{
			name:        "relative path",
			projectPath: "./my-project",
			wantBase:    "my-project",
			wantDroplet: "my-project-app",
		},
		{
			name:        "current directory",
			projectPath: ".",
			wantBase:    ".",
			wantDroplet: "app-app", // "." sanitizes to "app", then "-app" is added
		},
		{
			name:        "path with special chars",
			projectPath: "/home/user/projects/my@app!2024",
			wantBase:    "my@app!2024",
			wantDroplet: "myapp2024-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what the code does: extract base, then sanitize
			base := filepath.Base(tt.projectPath)
			if base != tt.wantBase {
				t.Errorf("filepath.Base(%q) = %q, want %q", tt.projectPath, base, tt.wantBase)
			}

			sanitized := util.SanitizeHostname(base)
			dropletName := sanitized + "-app"

			if dropletName != tt.wantDroplet {
				t.Errorf("Droplet name = %q, want %q (from path %q)", dropletName, tt.wantDroplet, tt.projectPath)
			}

			t.Logf("Path %q → Base %q → Sanitized %q → Droplet %q", tt.projectPath, base, sanitized, dropletName)
		})
	}
}

func TestSanitizeHostname_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "only dots",
			input: "...",
			want:  "app",
		},
		{
			name:  "only hyphens",
			input: "---",
			want:  "app",
		},
		{
			name:  "mixed dots and hyphens",
			input: ".-.-.",
			want:  "app",
		},
		{
			name:  "very long name",
			input: "this_is_a_very_long_project_name_with_many_underscores_and_special_characters",
			want:  "this-is-a-very-long-project-name-with-many-underscores-and-special-characters",
		},
		{
			name:  "single character valid",
			input: "a",
			want:  "a",
		},
		{
			name:  "single character invalid",
			input: "@",
			want:  "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.SanitizeHostname(tt.input)
			if got != tt.want {
				t.Errorf("util.SanitizeHostname(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
