package detector

import (
	"fmt"
	"lightfold/pkg/config"
)

func djangoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
		"python manage.py collectstatic --noinput",
	}
	run := []string{
		"gunicorn <yourproject>.wsgi:application --bind 0.0.0.0:8000 --workers 2",
	}
	health := map[string]any{"path": "/healthz", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{
		"DJANGO_SETTINGS_MODULE",
		"SECRET_KEY",
		"DATABASE_URL",
		"ALLOWED_HOSTS",
	}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func nextPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{getJSStartCommand(pm)}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NEXT_PUBLIC_*, any server-only envs"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func astroPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getPreviewCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PUBLIC_*, any server-only envs for SSR"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func gatsbyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"GATSBY_*, any build-time envs"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func sveltePlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getPreviewCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PUBLIC_*, any server-only envs for SvelteKit SSR"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func vuePlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getPreviewCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"VUE_APP_*, VITE_* for Vite-based setups"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func angularPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getJSStartCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NG_APP_*, any environment-specific configs"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func flaskPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
	}
	run := []string{
		"gunicorn --bind 0.0.0.0:5000 --workers 2 app:app",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"FLASK_ENV", "FLASK_APP", "DATABASE_URL", "SECRET_KEY"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func expressPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		fmt.Sprintf("%s", getJSBuildCommand(pm)),
	}
	run := []string{
		"node server.js",
		fmt.Sprintf("%s", getJSStartCommand(pm)),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func fastApiPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
	}
	run := []string{
		"uvicorn main:app --host 0.0.0.0 --port 8000",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"DATABASE_URL", "SECRET_KEY", "DEBUG"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func springBootPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	var build []string
	var buildTool string
	if fileExists(root, "pom.xml") {
		build = []string{
			"./mvnw clean package -DskipTests",
		}
		buildTool = "maven"
	} else {
		build = []string{
			"./gradlew build -x test",
		}
		buildTool = "gradle"
	}
	run := []string{
		"java -jar target/*.jar",
	}
	health := map[string]any{"path": "/actuator/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"SPRING_PROFILES_ACTIVE", "DATABASE_URL", "SERVER_PORT"}
	meta := map[string]string{"build_tool": buildTool}
	return build, run, health, env, meta
}

func aspNetPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"dotnet restore",
		"dotnet publish -c Release -o out",
	}
	run := []string{
		"dotnet out/*.dll",
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"ASPNETCORE_ENVIRONMENT", "ConnectionStrings__DefaultConnection"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

func phoenixPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func nestjsPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		"node dist/main",
		getRunCommand(pm, "start:prod"),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func laravelPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func railsPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"bundle install --deployment --without development test",
		"bundle exec rails db:migrate",
		"bundle exec rails assets:precompile",
	}
	run := []string{
		"bundle exec puma -C config/puma.rb",
	}
	health := map[string]any{"path": "/up", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"RAILS_ENV", "DATABASE_URL", "SECRET_KEY_BASE"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

func goPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"go build -o app ./...",
	}
	run := []string{
		"./app -port 8080",
	}
	health := map[string]any{"path": "/healthz", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PORT", "any app-specific envs"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

func dockerPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{"docker build -t app:latest ."}
	run := []string{"docker run -p 8080:8080 app:latest"}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{}
	return build, run, health, env, meta
}

func detectPackageManager(root string) string {
	switch {
	case fileExists(root, "bun.lockb") || fileExists(root, "bun.lock"):
		return "bun"
	case fileExists(root, "pnpm-lock.yaml"):
		return "pnpm"
	case fileExists(root, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

func detectPythonPackageManager(root string) string {
	switch {
	case fileExists(root, "uv.lock"):
		return "uv"
	case fileExists(root, "poetry.lock"):
		return "poetry"
	case fileExists(root, "Pipfile.lock"):
		return "pipenv"
	default:
		return "pip"
	}
}

func getJSInstallCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun install"
	case "pnpm":
		return "pnpm install"
	case "yarn":
		return "yarn install"
	default:
		return "npm install"
	}
}

func getJSBuildCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun run build"
	case "pnpm":
		return "pnpm run build"
	case "yarn":
		return "yarn build"
	default:
		return "npm run build"
	}
}

func getJSStartCommand(pm string) string {
	switch pm {
	case "bun":
		return "bun run start"
	case "pnpm":
		return "pnpm start"
	case "yarn":
		return "yarn start"
	default:
		return "npm start"
	}
}

func getPythonInstallCommand(pm string) string {
	switch pm {
	case "uv":
		return "uv sync"
	case "poetry":
		return "poetry install"
	case "pipenv":
		return "pipenv install"
	default:
		return "pip install -r requirements.txt"
	}
}

func getPreviewCommand(pm string) string {
	switch pm {
	case "npm":
		return "npm run preview"
	default:
		return pm + " run preview"
	}
}

func getRunCommand(pm string, script string) string {
	switch pm {
	case "npm":
		return "npm run " + script
	default:
		return pm + " run " + script
	}
}