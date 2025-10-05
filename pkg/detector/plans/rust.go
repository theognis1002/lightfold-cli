package plans

import (
	"lightfold/pkg/config"
)

// ActixPlan returns the build and run plan for Actix
func ActixPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"cargo build --release",
	}
	run := []string{
		"./target/release/$(grep '^name' Cargo.toml | head -1 | cut -d'\"' -f2 | tr -d ' ')",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"RUST_LOG", "PORT"}
	meta := map[string]string{"build_output": "target/release/"}
	return build, run, health, env, meta
}

// AxumPlan returns the build and run plan for Axum
func AxumPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"cargo build --release",
	}
	run := []string{
		"./target/release/$(grep '^name' Cargo.toml | head -1 | cut -d'\"' -f2 | tr -d ' ')",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"RUST_LOG", "PORT", "DATABASE_URL"}
	meta := map[string]string{"build_output": "target/release/"}
	return build, run, health, env, meta
}
