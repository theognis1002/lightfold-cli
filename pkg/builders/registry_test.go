package builders_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lightfold/pkg/builders"
	_ "lightfold/pkg/builders/dockerfile" // Register dockerfile builder
	_ "lightfold/pkg/builders/native"     // Register native builder
	_ "lightfold/pkg/builders/nixpacks"   // Register nixpacks builder
	"lightfold/pkg/detector"
)

func TestRegister(t *testing.T) {
	// Test that we can get registered builders
	// Note: Builders are registered via init() when packages are imported

	builder, err := builders.GetBuilder("native")
	if err != nil {
		t.Fatalf("Expected builder to be registered, got error: %v", err)
	}

	if builder.Name() != "native" {
		t.Errorf("Expected builder name 'native', got '%s'", builder.Name())
	}
}

func TestGetBuilder(t *testing.T) {
	tests := []struct {
		name        string
		builderName string
		wantErr     bool
	}{
		{
			name:        "get native builder",
			builderName: "native",
			wantErr:     false,
		},
		{
			name:        "get nixpacks builder",
			builderName: "nixpacks",
			wantErr:     false,
		},
		{
			name:        "get dockerfile builder",
			builderName: "dockerfile",
			wantErr:     false,
		},
		{
			name:        "get unknown builder",
			builderName: "unknown",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := builders.GetBuilder(tt.builderName)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if builder.Name() != tt.builderName {
				t.Errorf("Expected builder name '%s', got '%s'", tt.builderName, builder.Name())
			}
		})
	}
}

func TestListAvailableBuilders(t *testing.T) {
	availableBuilders := builders.ListAvailableBuilders()

	// Native should always be available
	found := false
	for _, name := range availableBuilders {
		if name == "native" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'native' builder to be in available builders list")
	}

	// Should have at least 1 builder (native is always available)
	if len(availableBuilders) < 1 {
		t.Error("Expected at least 1 available builder")
	}
}

func TestAutoSelectBuilder_Dockerfile(t *testing.T) {
	// Create temp directory with Dockerfile
	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:18\n"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	detection := &detector.Detection{
		Language: "JavaScript",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if builderName != "dockerfile" {
		t.Errorf("Expected 'dockerfile' builder for project with Dockerfile, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Nixpacks_JavaScript(t *testing.T) {
	// Create temp directory without Dockerfile
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "JavaScript",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should select nixpacks if available, otherwise native
	// We can't guarantee nixpacks is installed, so accept both
	if builderName != "nixpacks" && builderName != "native" {
		t.Errorf("Expected 'nixpacks' or 'native' builder for JavaScript project, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Nixpacks_TypeScript(t *testing.T) {
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "TypeScript",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should select nixpacks if available, otherwise native
	if builderName != "nixpacks" && builderName != "native" {
		t.Errorf("Expected 'nixpacks' or 'native' builder for TypeScript project, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Nixpacks_Python(t *testing.T) {
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "Python",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should select nixpacks if available, otherwise native
	if builderName != "nixpacks" && builderName != "native" {
		t.Errorf("Expected 'nixpacks' or 'native' builder for Python project, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Native_Go(t *testing.T) {
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "Go",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Go projects should use native builder (not supported by nixpacks auto-select)
	if builderName != "native" {
		t.Errorf("Expected 'native' builder for Go project, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Native_PHP(t *testing.T) {
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "PHP",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// PHP projects should use native builder
	if builderName != "native" {
		t.Errorf("Expected 'native' builder for PHP project, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Priority_DockerfileOverNixpacks(t *testing.T) {
	// Create temp directory with Dockerfile
	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:18\n"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	detection := &detector.Detection{
		Language: "JavaScript", // Would normally trigger nixpacks
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Dockerfile should take priority over nixpacks
	if builderName != "dockerfile" {
		t.Errorf("Expected 'dockerfile' to take priority, got '%s'", builderName)
	}
}

func TestAutoSelectBuilder_Fallback_Native(t *testing.T) {
	tmpDir := t.TempDir()

	detection := &detector.Detection{
		Language: "Unknown",
	}

	builderName, err := builders.AutoSelectBuilder(tmpDir, detection)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should fallback to native for unknown languages
	if builderName != "native" {
		t.Errorf("Expected 'native' fallback for unknown language, got '%s'", builderName)
	}
}

// Mock builder for testing
type mockBuilder struct {
	name      string
	available bool
}

func (m *mockBuilder) Name() string      { return m.name }
func (m *mockBuilder) IsAvailable() bool { return m.available }
func (m *mockBuilder) Build(ctx context.Context, opts *builders.BuildOptions) (*builders.BuildResult, error) {
	return &builders.BuildResult{Success: true}, nil
}
func (m *mockBuilder) NeedsNginx() bool { return true }
