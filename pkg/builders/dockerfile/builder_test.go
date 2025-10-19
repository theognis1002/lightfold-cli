package dockerfile

import (
	"context"
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
	// IsAvailable() checks if Docker daemon is accessible
	// This will return true if Docker is installed and running
	available := builder.IsAvailable()
	t.Logf("Docker available: %v", available)
	// We don't assert a specific value since it depends on the test environment
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

	// Now that it's implemented, it should error because Dockerfile is missing
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
	t.Skip("Skipping full build test - requires Docker daemon and SSH executor")

	// Note: Full integration testing is covered by end-to-end tests
	// Unit test just verifies Dockerfile existence checking in TestDockerfileBuilder_Build_MissingDockerfile
}
