package builders

import (
	"context"
	"lightfold/pkg/detector"
	sshpkg "lightfold/pkg/ssh"
)

// Builder defines the interface for all build strategies
type Builder interface {
	// Name returns the builder identifier (e.g., "native", "nixpacks", "dockerfile")
	Name() string

	// IsAvailable checks if this builder can be used in the current environment
	IsAvailable() bool

	// Build executes the build process for a release
	Build(ctx context.Context, opts *BuildOptions) (*BuildResult, error)

	// NeedsNginx returns true if nginx reverse proxy setup is required
	// Returns false if the builder's output already includes a web server
	NeedsNginx() bool
}

// BuildOptions contains all parameters needed for a build operation
type BuildOptions struct {
	ProjectPath string                // Local project directory path
	Detection   *detector.Detection   // Framework detection results
	ReleasePath string                // Remote release directory path
	EnvVars     map[string]string     // Environment variables for build
	SSHExecutor *sshpkg.Executor      // SSH connection to remote server
}

// BuildResult contains the output of a build operation
type BuildResult struct {
	Success       bool   // Whether the build completed successfully
	BuildLog      string // Build output logs
	IncludesNginx bool   // Whether the build output includes nginx
	StartCommand  string // Optional: Command to start the application (for systemd)
}
