package plans

import (
	"lightfold/pkg/config"
)

// LaravelPlan returns the build and run plan for Laravel
func LaravelPlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"composer install --no-dev --optimize-autoloader",
		"php artisan migrate --force",
		"php artisan config:cache && php artisan route:cache",
	}
	run := []string{
		"php-fpm (with nginx)",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"APP_KEY", "APP_ENV", "DB_CONNECTION/DB_*"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

// SymfonyPlan returns the build and run plan for Symfony
func SymfonyPlan(fs FSReader) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"composer install --no-dev --optimize-autoloader",
		"php bin/console cache:clear --env=prod",
		"php bin/console assets:install",
	}
	run := []string{
		"php-fpm (with nginx)",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"APP_ENV", "APP_SECRET", "DATABASE_URL"}
	meta := map[string]string{}
	return build, run, health, env, meta
}
