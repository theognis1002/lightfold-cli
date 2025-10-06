package plans

import (
	"lightfold/pkg/config"
)

// AspNetPlan returns the build and run plan for ASP.NET
func AspNetPlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"dotnet restore",
		"dotnet publish -c Release -o out",
	}
	run := []string{
		"dotnet $(ls out/*.dll | head -n 1)",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"ASPNETCORE_ENVIRONMENT", "ConnectionStrings__DefaultConnection"}
	meta := map[string]string{"build_output": "out/"}
	return build, run, health, env, meta
}
