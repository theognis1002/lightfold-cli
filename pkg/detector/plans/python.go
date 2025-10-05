package plans

import (
	"lightfold/pkg/config"
	"lightfold/pkg/detector/packagemanagers"
)

// DjangoPlan returns the build and run plan for Django
func DjangoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectPython(root)
	serverType := packagemanagers.DetectDjangoServerType(root)

	build := []string{
		packagemanagers.GetPythonInstallCommand(pm),
		"python manage.py collectstatic --noinput",
	}

	run := []string{
		packagemanagers.GetDjangoRunCommand(serverType, ""),
	}

	health := map[string]any{"path": "/healthz", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{
		"DJANGO_SETTINGS_MODULE",
		"SECRET_KEY",
		"DATABASE_URL",
		"ALLOWED_HOSTS",
	}
	meta := map[string]string{
		"package_manager": pm,
		"server_type":     serverType,
	}
	return build, run, health, env, meta
}

// FlaskPlan returns the build and run plan for Flask
func FlaskPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectPython(root)
	build := []string{
		packagemanagers.GetPythonInstallCommand(pm),
	}
	run := []string{
		"gunicorn --bind 0.0.0.0:5000 --workers 2 app:app",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"FLASK_ENV", "FLASK_APP", "DATABASE_URL", "SECRET_KEY"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

// FastAPIPlan returns the build and run plan for FastAPI
func FastAPIPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectPython(root)
	build := []string{
		packagemanagers.GetPythonInstallCommand(pm),
	}
	run := []string{
		"uvicorn main:app --host 0.0.0.0 --port 8000",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"DATABASE_URL", "SECRET_KEY", "DEBUG"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}
