package plans

import (
	"lightfold/pkg/config"
)

// GinPlan returns the build and run plan for Gin
func GinPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"go build -o app .",
	}
	run := []string{
		"./app",
	}
	health := map[string]any{"path": "/ping", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"GIN_MODE", "PORT"}
	meta := map[string]string{"framework": "gin"}
	return build, run, health, env, meta
}

// EchoPlan returns the build and run plan for Echo
func EchoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"go build -o app .",
	}
	run := []string{
		"./app",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PORT", "DATABASE_URL"}
	meta := map[string]string{"framework": "echo"}
	return build, run, health, env, meta
}

// FiberPlan returns the build and run plan for Fiber
func FiberPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"go build -o app .",
	}
	run := []string{
		"./app",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PORT", "DATABASE_URL"}
	meta := map[string]string{"framework": "fiber"}
	return build, run, health, env, meta
}

// GoPlan returns the build and run plan for generic Go applications
func GoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"go build -o app .",
	}
	run := []string{
		"./app -port 8080",
	}
	health := map[string]any{"path": "/healthz", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PORT", "any app-specific envs"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

// HugoPlan returns the build and run plan for Hugo
func HugoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"hugo --minify",
	}
	run := []string{
		"hugo server --bind 0.0.0.0 --port 1313",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"HUGO_ENV"}
	meta := map[string]string{"build_output": "public/", "static": "true"}
	return build, run, health, env, meta
}
