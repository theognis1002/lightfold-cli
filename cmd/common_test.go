package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"lightfold/pkg/config"
	"lightfold/pkg/detector"
)

func TestResolveBuilder_FlagPriority(t *testing.T) {
	target := config.TargetConfig{
		Builder: "native", // Existing config
	}

	detection := &detector.Detection{
		Language: "JavaScript", // Would auto-select nixpacks
	}

	tmpDir := t.TempDir()

	// Test: CLI flag should override everything
	builderName := resolveBuilder(target, tmpDir, detection, "dockerfile")
	if builderName != "dockerfile" {
		t.Errorf("Expected CLI flag to take priority, got '%s'", builderName)
	}
}

func TestResolveBuilder_ConfigPriority(t *testing.T) {
	target := config.TargetConfig{
		Builder: "native", // Existing config
	}

	detection := &detector.Detection{
		Language: "JavaScript", // Would auto-select nixpacks
	}

	tmpDir := t.TempDir()

	// Test: Config should be used when flag is empty
	builderName := resolveBuilder(target, tmpDir, detection, "")
	if builderName != "native" {
		t.Errorf("Expected config builder to be used, got '%s'", builderName)
	}
}

func TestResolveBuilder_AutoSelect_Dockerfile(t *testing.T) {
	target := config.TargetConfig{
		Builder: "", // No existing builder
	}

	detection := &detector.Detection{
		Language: "JavaScript",
	}

	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:18\n"), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Test: Should auto-select dockerfile
	builderName := resolveBuilder(target, tmpDir, detection, "")
	if builderName != "dockerfile" {
		t.Errorf("Expected auto-select 'dockerfile', got '%s'", builderName)
	}
}

func TestResolveBuilder_AutoSelect_Nixpacks(t *testing.T) {
	target := config.TargetConfig{
		Builder: "", // No existing builder
	}

	detection := &detector.Detection{
		Language: "JavaScript",
	}

	tmpDir := t.TempDir()

	// Test: Should auto-select nixpacks (if available) or native
	builderName := resolveBuilder(target, tmpDir, detection, "")
	if builderName != "nixpacks" && builderName != "native" {
		t.Errorf("Expected 'nixpacks' or 'native', got '%s'", builderName)
	}
}

func TestResolveBuilder_AutoSelect_Native_Fallback(t *testing.T) {
	target := config.TargetConfig{
		Builder: "", // No existing builder
	}

	detection := &detector.Detection{
		Language: "Go", // Not supported by nixpacks auto-select
	}

	tmpDir := t.TempDir()

	// Test: Should fall back to native
	builderName := resolveBuilder(target, tmpDir, detection, "")
	if builderName != "native" {
		t.Errorf("Expected 'native' fallback, got '%s'", builderName)
	}
}

func TestResolveBuilder_UnavailableBuilder_Fallback(t *testing.T) {
	target := config.TargetConfig{
		Builder: "nonexistent-builder", // Invalid builder
	}

	detection := &detector.Detection{
		Language: "JavaScript",
	}

	tmpDir := t.TempDir()

	// Test: Should fall back to auto-select when configured builder is unavailable
	builderName := resolveBuilder(target, tmpDir, detection, "")

	// Should auto-select since configured builder doesn't exist
	if builderName == "" {
		t.Error("Expected a builder to be selected, got empty string")
	}

	// Should be native or nixpacks (depending on availability)
	if builderName != "native" && builderName != "nixpacks" {
		t.Errorf("Expected 'native' or 'nixpacks' after fallback, got '%s'", builderName)
	}
}

func TestResolveBuilder_Priority_Order(t *testing.T) {
	tests := []struct {
		name             string
		targetBuilder    string
		flagValue        string
		language         string
		hasDockerfile    bool
		expectedPriority string // What should be selected
	}{
		{
			name:             "flag overrides everything",
			targetBuilder:    "native",
			flagValue:        "nixpacks",
			language:         "Go",
			hasDockerfile:    true,
			expectedPriority: "nixpacks",
		},
		{
			name:             "config used when no flag",
			targetBuilder:    "native",
			flagValue:        "",
			language:         "JavaScript",
			hasDockerfile:    true,
			expectedPriority: "native",
		},
		{
			name:             "auto-select when no flag or config",
			targetBuilder:    "",
			flagValue:        "",
			language:         "JavaScript",
			hasDockerfile:    false,
			expectedPriority: "nixpacks or native", // Depends on nixpacks availability
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := config.TargetConfig{
				Builder: tt.targetBuilder,
			}

			detection := &detector.Detection{
				Language: tt.language,
			}

			tmpDir := t.TempDir()
			if tt.hasDockerfile {
				dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
				if err := os.WriteFile(dockerfilePath, []byte("FROM alpine\n"), 0644); err != nil {
					t.Fatalf("Failed to create Dockerfile: %v", err)
				}
			}

			builderName := resolveBuilder(target, tmpDir, detection, tt.flagValue)

			if tt.expectedPriority == "nixpacks or native" {
				if builderName != "nixpacks" && builderName != "native" {
					t.Errorf("Expected 'nixpacks' or 'native', got '%s'", builderName)
				}
			} else {
				if builderName != tt.expectedPriority {
					t.Errorf("Expected '%s', got '%s'", tt.expectedPriority, builderName)
				}
			}
		})
	}
}

func TestResolveBuilder_EmptyDetection(t *testing.T) {
	target := config.TargetConfig{
		Builder: "",
	}

	detection := &detector.Detection{
		Language: "",
	}

	tmpDir := t.TempDir()

	// Should not panic and should return native
	builderName := resolveBuilder(target, tmpDir, detection, "")
	if builderName != "native" {
		t.Errorf("Expected 'native' for empty detection, got '%s'", builderName)
	}
}

func TestResolveBuilder_NilDetection(t *testing.T) {
	target := config.TargetConfig{
		Builder: "",
	}

	tmpDir := t.TempDir()

	// Should handle nil detection gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("resolveBuilder panicked with nil detection: %v", r)
		}
	}()

	builderName := resolveBuilder(target, tmpDir, nil, "")
	if builderName == "" {
		t.Error("Expected non-empty builder name even with nil detection")
	}
}
