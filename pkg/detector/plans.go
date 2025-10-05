package detector

import (
	"os"
	"path/filepath"
	"strings"

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

	if dirExists(root, "app") {
		meta["router"] = "app"
	} else if dirExists(root, "pages") {
		meta["router"] = "pages"
	}

	meta["build_output"] = ".next/"
	if fileExists(root, "next.config.js") {
		configBytes, _ := os.ReadFile(filepath.Join(root, "next.config.js"))
		configContent := string(configBytes)
		if strings.Contains(configContent, "output: 'export'") ||
			strings.Contains(configContent, `output: "export"`) {
			meta["export"] = "static"
			meta["build_output"] = "out/"
		}
	}
	if fileExists(root, "next.config.ts") {
		configBytes, _ := os.ReadFile(filepath.Join(root, "next.config.ts"))
		configContent := string(configBytes)
		if strings.Contains(configContent, "output: 'export'") ||
			strings.Contains(configContent, `output: "export"`) {
			meta["export"] = "static"
			meta["build_output"] = "out/"
		}
	}

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
	meta := map[string]string{"package_manager": pm, "build_output": "dist/"}
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
	meta := map[string]string{"package_manager": pm, "build_output": "public/"}
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
	meta := map[string]string{"package_manager": pm, "build_output": "build/"}
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
	meta := map[string]string{"package_manager": pm, "build_output": "dist/"}

	if fileExists(root, "package.json") {
		packageJSON := readFile(root, "package.json")
		if strings.Contains(packageJSON, `"vue"`) {
			if strings.Contains(packageJSON, `"vue": "^3.`) || strings.Contains(packageJSON, `"vue": "~3.`) || strings.Contains(packageJSON, `"vue": "3.`) {
				meta["vue_version"] = "3"
			} else if strings.Contains(packageJSON, `"vue": "^2.`) || strings.Contains(packageJSON, `"vue": "~2.`) || strings.Contains(packageJSON, `"vue": "2.`) {
				meta["vue_version"] = "2"
			}
		}
	}

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
	meta := map[string]string{"package_manager": pm, "build_output": "dist/"}
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
	// Check for Deno runtime first
	if detectDenoRuntime(root) {
		build := []string{
			"deno cache main.ts",
		}
		run := []string{
			"deno run --allow-net --allow-read --allow-env main.ts",
		}
		health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
		env := []string{"PORT", "DATABASE_URL"}
		meta := map[string]string{"runtime": "deno"}
		return build, run, health, env, meta
	}

	// Node.js runtime
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
	}
	run := []string{
		getJSStartCommand(pm),
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
	var run []string
	var buildTool string
	var buildOutput string
	if fileExists(root, "pom.xml") {
		build = []string{
			"./mvnw clean package -DskipTests",
		}
		run = []string{
			"java -jar target/*.jar",
		}
		buildTool = "maven"
		buildOutput = "target/"
	} else {
		build = []string{
			"./gradlew build -x test",
		}
		run = []string{
			"java -jar build/libs/*.jar",
		}
		buildTool = "gradle"
		buildOutput = "build/"
	}
	health := map[string]any{"path": "/actuator/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"SPRING_PROFILES_ACTIVE", "DATABASE_URL", "SERVER_PORT"}
	meta := map[string]string{"build_tool": buildTool, "build_output": buildOutput}
	return build, run, health, env, meta
}

func aspNetPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func dockerPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{"docker build -t app:latest ."}
	run := []string{"docker run -p 8080:8080 app:latest"}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{}
	return build, run, health, env, meta
}

func detectDenoRuntime(root string) bool {
	return fileExists(root, "deno.json") || fileExists(root, "deno.jsonc")
}

func detectPackageManager(root string) string {
	switch {
	case fileExists(root, "bun.lockb") || fileExists(root, "bun.lock"):
		return "bun"
	case fileExists(root, ".yarnrc.yml"):
		return "yarn-berry"
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
	case fileExists(root, "pdm.lock"):
		return "pdm"
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
	case "yarn", "yarn-berry":
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
	case "yarn", "yarn-berry":
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
	case "yarn", "yarn-berry":
		return "yarn start"
	default:
		return "npm start"
	}
}

func getPythonInstallCommand(pm string) string {
	switch pm {
	case "uv":
		return "uv sync"
	case "pdm":
		return "pdm install --prod"
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

func remixPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getJSStartCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "SESSION_SECRET"}
	meta := map[string]string{"package_manager": pm, "build_output": "build/"}
	return build, run, health, env, meta
}

func nuxtPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		"node .output/server/index.mjs",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NUXT_PUBLIC_*", "NITRO_*"}
	meta := map[string]string{"package_manager": pm, "build_output": ".output/"}
	return build, run, health, env, meta
}

func symfonyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func fastifyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	// Check for Deno runtime first
	if detectDenoRuntime(root) {
		build := []string{
			"deno cache main.ts",
		}
		run := []string{
			"deno run --allow-net --allow-read --allow-env main.ts",
		}
		health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
		env := []string{"PORT", "DATABASE_URL"}
		meta := map[string]string{"runtime": "deno"}
		return build, run, health, env, meta
	}

	// Node.js runtime
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
	}
	run := []string{
		getJSStartCommand(pm),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

func ginPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func echoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func fiberPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func hugoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func eleventyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getRunCommand(pm, "build"),
	}
	run := []string{
		getRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"ELEVENTY_ENV"}
	meta := map[string]string{"package_manager": pm, "build_output": "_site/", "static": "true"}
	return build, run, health, env, meta
}

func jekyllPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"bundle install",
		"bundle exec jekyll build",
	}
	run := []string{
		"bundle exec jekyll serve --host 0.0.0.0",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"JEKYLL_ENV"}
	meta := map[string]string{"build_output": "_site/", "static": "true"}
	return build, run, health, env, meta
}

func docusaurusPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := detectPackageManager(root)
	build := []string{
		getJSInstallCommand(pm),
		getJSBuildCommand(pm),
	}
	run := []string{
		getRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{"package_manager": pm, "build_output": "build/"}
	return build, run, health, env, meta
}

func actixPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func axumPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
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

func readFile(root, rel string) string {
	b, _ := os.ReadFile(filepath.Join(root, rel))
	return string(b)
}