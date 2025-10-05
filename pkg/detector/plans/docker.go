package plans

import (
	"lightfold/pkg/config"
)

// DockerPlan returns the build and run plan for Docker-based applications
func DockerPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{"docker build -t app:latest ."}
	run := []string{"docker run -p 8080:8080 app:latest"}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{}
	return build, run, health, env, meta
}
