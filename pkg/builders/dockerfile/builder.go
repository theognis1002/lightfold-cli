package dockerfile

import (
	"context"
	"fmt"
	"lightfold/pkg/builders"
	"os"
	"path/filepath"
)

// DockerfileBuilder uses an existing Dockerfile for building
type DockerfileBuilder struct{}

func init() {
	builders.Register("dockerfile", func() builders.Builder {
		return &DockerfileBuilder{}
	})
}

func (d *DockerfileBuilder) Name() string {
	return "dockerfile"
}

func (d *DockerfileBuilder) IsAvailable() bool {
	return true
}

func (d *DockerfileBuilder) NeedsNginx() bool {
	return false
}

func (d *DockerfileBuilder) Build(ctx context.Context, opts *builders.BuildOptions) (*builders.BuildResult, error) {
	dockerfilePath := filepath.Join(opts.ProjectPath, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return &builders.BuildResult{
			Success: false,
		}, fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
	}

	// TODO: Implement dockerfile builder
	// - Build image: docker build -t <app>:latest .
	// - Export as tarball: docker save <app>:latest -o /tmp/image.tar
	// - Transfer to server via SSH
	// - Load and run: docker load < image.tar && docker run ...

	return &builders.BuildResult{
		Success:       false,
		BuildLog:      "",
		IncludesNginx: true,
	}, fmt.Errorf("dockerfile builder not yet implemented - requires container runtime support")
}
