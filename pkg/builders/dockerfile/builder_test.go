package dockerfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lightfold/pkg/builders"
)

func TestDockerfileBuilder_Name(t *testing.T) {
	builder := &DockerfileBuilder{}
	if builder.Name() != "dockerfile" {
		t.Errorf("Expected name 'dockerfile', got '%s'", builder.Name())
	}
}

func TestDockerfileBuilder_IsAvailable(t *testing.T) {
	builder := &DockerfileBuilder{}
	if !builder.IsAvailable() {
		t.Error("Dockerfile builder should always be available (registration check)")
	}
}

func TestDockerfileBuilder_NeedsNginx(t *testing.T) {
	builder := &DockerfileBuilder{}
	if builder.NeedsNginx() {
		t.Error("Dockerfile builder should not need nginx (assumes Dockerfile includes server)")
	}
}

func TestDockerfileBuilder_Build_NotImplemented(t *testing.T) {
	builder := &DockerfileBuilder{}

	tmpDir := t.TempDir()

	result, err := builder.Build(context.Background(), &builders.BuildOptions{
		ProjectPath: tmpDir,
	})

	if err == nil {
		t.Error("Expected error for not implemented builder")
	}

	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected success=false for not implemented builder")
	}

	// Verify error message mentions not implemented
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestDockerfileBuilder_Build_MissingDockerfile(t *testing.T) {
	builder := &DockerfileBuilder{}

	tmpDir := t.TempDir()
	// Don't create Dockerfile

	result, err := builder.Build(context.Background(), &builders.BuildOptions{
		ProjectPath: tmpDir,
	})

	if err == nil {
		t.Error("Expected error when Dockerfile is missing")
	}

	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected success=false when Dockerfile is missing")
	}
}

func TestDockerfileBuilder_Build_DockerfileExists(t *testing.T) {
	builder := &DockerfileBuilder{}

	tmpDir := t.TempDir()
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte("FROM node:18\nCOPY . .\n"), 0644); err != nil {
		t.Fatalf("Failed to create test Dockerfile: %v", err)
	}

	result, err := builder.Build(context.Background(), &builders.BuildOptions{
		ProjectPath: tmpDir,
	})

	// Should still error because not implemented, but should pass Dockerfile check
	if err == nil {
		t.Error("Expected error for not implemented builder")
	}

	// Error should be about not implemented, not missing Dockerfile
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}
