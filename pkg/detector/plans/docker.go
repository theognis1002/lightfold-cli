package plans

import (
	"lightfold/pkg/config"
)

// DockerPlan returns the build and run plan for Docker-based applications
func DockerPlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{"docker build -t app:latest ."}
	run := []string{"docker run -p 8080:8080 app:latest"}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{}
	return build, run, health, env, meta
}

// DockerComposePlan returns the build and run plan for Docker Compose applications
// Uses Docker Compose V2 syntax (docker compose) which is the modern standard
func DockerComposePlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{"docker compose build"}
	run := []string{"docker compose up -d"}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{"deployment_type": "docker-compose"}
	return build, run, health, env, meta
}
