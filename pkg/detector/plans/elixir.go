package plans

import (
	"lightfold/pkg/config"
)

// PhoenixPlan returns the build and run plan for Phoenix
func PhoenixPlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"mix deps.get",
		"mix compile",
		"mix assets.deploy",
		"mix phx.digest",
	}
	run := []string{
		"mix phx.server",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"DATABASE_URL", "SECRET_KEY_BASE", "PHX_HOST"}
	meta := map[string]string{}
	return build, run, health, env, meta
}
