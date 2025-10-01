package detector

import "fmt"

// ------- Plans (opinionated defaults) -------

func djangoPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
		"python manage.py collectstatic --noinput",
		"# (optional) run tests",
	}
	run := []string{
		"gunicorn <yourproject>.wsgi:application --bind 0.0.0.0:8000 --workers 2",
	}
	health := map[string]any{"path": "/healthz", "expect": 200, "timeout_seconds": 30}
	env := []string{
		"DJANGO_SETTINGS_MODULE",
		"SECRET_KEY",
		"DATABASE_URL",
		"ALLOWED_HOSTS",
	}
	return build, run, health, env
}

func nextPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		"next build",
		"# if using static export in next.config: next export",
	}
	run := []string{"next start -p 3000"}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"NEXT_PUBLIC_*, any server-only envs"}
	return build, run, health, env
}

func astroPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		"astro build",
	}
	run := []string{
		"astro preview --port 4321",
		"# or serve static files from dist/ with any static server",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"PUBLIC_*, any server-only envs for SSR"}
	return build, run, health, env
}

func gatsbyPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		"gatsby build",
	}
	run := []string{
		"gatsby serve -p 9000",
		"# or serve static files from public/ with any static server",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"GATSBY_*, any build-time envs"}
	return build, run, health, env
}

func sveltePlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getPreviewCommand(pm),
		"# or for SvelteKit: node build",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"PUBLIC_*, any server-only envs for SvelteKit SSR"}
	return build, run, health, env
}

func vuePlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getPreviewCommand(pm),
		"# or serve dist/ with any static server",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"VUE_APP_*, VITE_* for Vite-based setups"}
	return build, run, health, env
}

func angularPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		"ng build --configuration production",
	}
	run := []string{
		"ng serve --port 4200 --host 0.0.0.0",
		"# or serve dist/ with any static server",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"NG_APP_*, any environment-specific configs"}
	return build, run, health, env
}

func flaskPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
		"# optional: flask db upgrade if using Flask-Migrate",
	}
	run := []string{
		"gunicorn --bind 0.0.0.0:5000 --workers 2 app:app",
		"# or flask run --host=0.0.0.0 --port=5000 for development",
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"FLASK_ENV", "FLASK_APP", "DATABASE_URL", "SECRET_KEY"}
	return build, run, health, env
}

func expressPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		fmt.Sprintf("# optional: %s if using TypeScript or bundling", getJSBuildCommand(pm)),
	}
	run := []string{
		"node server.js",
		fmt.Sprintf("# or %s if defined in package.json", getJSStartCommand(pm)),
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	return build, run, health, env
}

func fastApiPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPythonPackageManager(root)
	build := []string{
		getPythonInstallCommand(pm),
		"# optional: alembic upgrade head if using database migrations",
	}
	run := []string{
		"uvicorn main:app --host 0.0.0.0 --port 8000",
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"DATABASE_URL", "SECRET_KEY", "DEBUG"}
	return build, run, health, env
}

func springBootPlan(root string) ([]string, []string, map[string]any, []string) {
	var build []string
	if fileExists(root, "pom.xml") {
		build = []string{
			"./mvnw clean package -DskipTests",
		}
	} else {
		build = []string{
			"./gradlew build -x test",
		}
	}
	run := []string{
		"java -jar target/*.jar",
		"# or java -jar build/libs/*.jar for Gradle",
	}
	health := map[string]any{"path": "/actuator/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"SPRING_PROFILES_ACTIVE", "DATABASE_URL", "SERVER_PORT"}
	return build, run, health, env
}

func aspNetPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{
		"dotnet restore",
		"dotnet publish -c Release -o out",
	}
	run := []string{
		"dotnet out/*.dll",
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"ASPNETCORE_ENVIRONMENT", "ConnectionStrings__DefaultConnection"}
	return build, run, health, env
}

func phoenixPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{
		"mix deps.get",
		"mix compile",
		"mix assets.deploy",
		"mix phx.digest",
	}
	run := []string{
		"mix phx.server",
	}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{"DATABASE_URL", "SECRET_KEY_BASE", "PHX_HOST"}
	return build, run, health, env
}

func nestjsPlan(root string) ([]string, []string, map[string]any, []string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		"node dist/main",
		"# or " + getRunCommand(pm, "start:prod"),
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	return build, run, health, env
}

func laravelPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{
		"composer install --no-dev --optimize-autoloader",
		"# optional front-end: npm ci && npm run build",
		"php artisan migrate --force",
		"php artisan config:cache && php artisan route:cache",
	}
	run := []string{
		"php-fpm (with nginx)  # or your preferred process manager",
	}
	health := map[string]any{"path": "/health", "expect": 200, "timeout_seconds": 30}
	env := []string{"APP_KEY", "APP_ENV", "DB_CONNECTION/DB_*"}
	return build, run, health, env
}

func railsPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{
		"bundle install --deployment --without development test",
		"bundle exec rails db:migrate",
		"bundle exec rails assets:precompile",
	}
	run := []string{
		"bundle exec puma -C config/puma.rb",
	}
	health := map[string]any{"path": "/up", "expect": 200, "timeout_seconds": 30}
	env := []string{"RAILS_ENV", "DATABASE_URL", "SECRET_KEY_BASE"}
	return build, run, health, env
}

func goPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{
		"go build -o app ./...",
	}
	run := []string{
		"./app -port 8080",
	}
	health := map[string]any{"path": "/healthz", "expect": 200, "timeout_seconds": 30}
	env := []string{"PORT", "any app-specific envs"}
	return build, run, health, env
}

func dockerPlan(root string) ([]string, []string, map[string]any, []string) {
	build := []string{"docker build -t app:latest ."}
	run := []string{"docker run -p 8080:8080 app:latest"}
	health := map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30}
	env := []string{}
	return build, run, health, env
}

func detectPackageManager(root string) string {
	switch {
	case fileExists(root, "bun.lockb"):
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