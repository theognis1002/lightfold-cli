package detector

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Detection struct {
	Framework   string            `json:"framework"`
	Language    string            `json:"language"`
	Confidence  float64           `json:"confidence"`
	Signals     []string          `json:"signals"`
	BuildPlan   []string          `json:"build_plan"`
	RunPlan     []string          `json:"run_plan"`
	Healthcheck map[string]any    `json:"healthcheck"`
	EnvSchema   []string          `json:"env_schema"`
	Meta        map[string]string `json:"meta,omitempty"`
}

type candidate struct {
	name     string
	score    float64
	language string
	signals  []string
	plan     func(root string) (build []string, run []string, health map[string]any, env []string, meta map[string]string)
}

func DetectFramework(root string) Detection {
	allFiles, extCounts, err := scanTree(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		os.Exit(1)
	}
	has := func(rel string) bool { return fileExists(root, rel) }
	read := func(rel string) string { b, _ := os.ReadFile(filepath.Join(root, rel)); return string(b) }

	var cands []candidate

	{
		score := 0.0
		signals := []string{}
		if has("manage.py") {
			score += 3
			signals = append(signals, "manage.py")
		}
		if has("requirements.txt") || has("Pipfile.lock") || has("poetry.lock") || has("pyproject.toml") {
			score += 1.5
			signals = append(signals, "python deps lockfile")
		}
		if has("myproject/wsgi.py") || has("wsgi.py") || has("asgi.py") {
			score += 1
			signals = append(signals, "wsgi/asgi")
		}
		if strings.Contains(strings.ToLower(read("requirements.txt")), "django") || strings.Contains(strings.ToLower(read("pyproject.toml")), "django") {
			score += 1.5
			signals = append(signals, "mentions django in deps")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Django",
				score:    score,
				language: "Python",
				signals:  signals,
				plan:     djangoPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("next.config.js") || has("next.config.ts") {
			score += 2.5
			signals = append(signals, "next.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"next"`) {
				score += 2.5
				signals = append(signals, "package.json has next")
			}
			if strings.Contains(pj, `"scripts"`) && (strings.Contains(pj, `"next build"`) || strings.Contains(pj, `"next dev"`)) {
				score += 1
				signals = append(signals, "package.json scripts for next")
			}
		}
		if dirExists(root, "pages") || dirExists(root, "app") {
			score += 0.5
			signals = append(signals, "pages/ or app/ folder")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Next.js",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     nextPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("remix.config.js") || has("remix.config.ts") {
			score += 3
			signals = append(signals, "remix.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@remix-run/react"`) {
				score += 2.5
				signals = append(signals, "package.json has @remix-run/react")
			}
		}
		if dirExists(root, "app/routes") {
			score += 1
			signals = append(signals, "app/routes/ directory")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Remix",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     remixPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("nuxt.config.js") || has("nuxt.config.ts") {
			score += 3
			signals = append(signals, "nuxt.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"nuxt"`) {
				score += 2.5
				signals = append(signals, "package.json has nuxt")
			}
		}
		if dirExists(root, "pages") || has("app.vue") {
			score += 1
			signals = append(signals, "pages/ directory or app.vue")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Nuxt.js",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     nuxtPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("astro.config.mjs") || has("astro.config.js") || has("astro.config.ts") {
			score += 3
			signals = append(signals, "astro.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"astro"`) {
				score += 2.5
				signals = append(signals, "package.json has astro")
			}
			if strings.Contains(pj, `"scripts"`) && (strings.Contains(pj, `"astro build"`) || strings.Contains(pj, `"astro dev"`)) {
				score += 1
				signals = append(signals, "package.json scripts for astro")
			}
		}
		if dirExists(root, "src") && dirExists(root, "public") {
			score += 0.5
			signals = append(signals, "src/ and public/ folders")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Astro",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     astroPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("gatsby-config.js") || has("gatsby-config.ts") {
			score += 3
			signals = append(signals, "gatsby-config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"gatsby"`) {
				score += 2.5
				signals = append(signals, "package.json has gatsby")
			}
			if strings.Contains(pj, `"gatsby build"`) || strings.Contains(pj, `"gatsby develop"`) {
				score += 1
				signals = append(signals, "package.json scripts for gatsby")
			}
		}
		if dirExists(root, "src/pages") {
			score += 1
			signals = append(signals, "src/pages/ folder")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Gatsby",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     gatsbyPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("svelte.config.js") || has("svelte.config.ts") {
			score += 3
			signals = append(signals, "svelte.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@sveltejs/kit"`) {
				score += 3
				signals = append(signals, "package.json has @sveltejs/kit")
			} else if strings.Contains(pj, `"svelte"`) {
				score += 2.5
				signals = append(signals, "package.json has svelte")
			}
		}
		if dirExists(root, "src/routes") {
			score += 1
			signals = append(signals, "src/routes/ folder (SvelteKit)")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Svelte",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     sveltePlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("vue.config.js") || has("vite.config.js") || has("vite.config.ts") {
			score += 2
			signals = append(signals, "vue/vite config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@vue/cli"`) || strings.Contains(pj, `"nuxt"`) {
				score += 3
				signals = append(signals, "Vue CLI or Nuxt")
			} else if strings.Contains(pj, `"vue"`) {
				score += 2.5
				signals = append(signals, "package.json has vue")
			}
		}
		if containsExt(allFiles, ".vue") {
			score += 2
			signals = append(signals, ".vue files")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Vue.js",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     vuePlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("angular.json") {
			score += 3
			signals = append(signals, "angular.json")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@angular/core"`) {
				score += 3
				signals = append(signals, "package.json has @angular/core")
			}
		}
		if has("tsconfig.json") && (dirExists(root, "src/app") || has("src/main.ts")) {
			score += 1
			signals = append(signals, "TypeScript config with Angular structure")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Angular",
				score:    score,
				language: "TypeScript",
				signals:  signals,
				plan:     angularPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("artisan") {
			score += 3
			signals = append(signals, "artisan")
		}
		if has("composer.lock") {
			score += 2
			signals = append(signals, "composer.lock")
		}
		if has("config/app.php") {
			score += 0.5
			signals = append(signals, "config/app.php")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Laravel",
				score:    score,
				language: "PHP",
				signals:  signals,
				plan:     laravelPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("symfony.lock") {
			score += 3
			signals = append(signals, "symfony.lock")
		}
		if has("bin/console") {
			score += 2.5
			signals = append(signals, "bin/console")
		}
		if has("composer.json") {
			composerContent := strings.ToLower(read("composer.json"))
			if strings.Contains(composerContent, "symfony") {
				score += 2
				signals = append(signals, "symfony in composer.json")
			}
		}
		if has("config/bundles.php") {
			score += 1
			signals = append(signals, "config/bundles.php")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Symfony",
				score:    score,
				language: "PHP",
				signals:  signals,
				plan:     symfonyPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("bin/rails") {
			score += 3
			signals = append(signals, "bin/rails")
		}
		if has("Gemfile.lock") {
			score += 2
			signals = append(signals, "Gemfile.lock")
		}
		if has("config/application.rb") {
			score += 0.5
			signals = append(signals, "config/application.rb")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Rails",
				score:    score,
				language: "Ruby",
				signals:  signals,
				plan:     railsPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("go.mod") {
			score += 2
			signals = append(signals, "go.mod")
			goMod := strings.ToLower(read("go.mod"))
			if strings.Contains(goMod, "github.com/gin-gonic/gin") {
				score += 2
				signals = append(signals, "github.com/gin-gonic/gin in go.mod")
			}
		}
		if containsExt(allFiles, ".go") {
			for _, file := range allFiles {
				if strings.HasSuffix(file, ".go") {
					content := strings.ToLower(read(file))
					if strings.Contains(content, "github.com/gin-gonic/gin") {
						score += 2
						signals = append(signals, "Gin import in .go files")
						break
					}
				}
			}
		}
		if score >= 4 {
			cands = append(cands, candidate{
				name:     "Gin",
				score:    score,
				language: "Go",
				signals:  signals,
				plan:     ginPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("go.mod") {
			score += 2
			signals = append(signals, "go.mod")
			goMod := strings.ToLower(read("go.mod"))
			if strings.Contains(goMod, "github.com/labstack/echo") {
				score += 2
				signals = append(signals, "github.com/labstack/echo in go.mod")
			}
		}
		if containsExt(allFiles, ".go") {
			for _, file := range allFiles {
				if strings.HasSuffix(file, ".go") {
					content := strings.ToLower(read(file))
					if strings.Contains(content, "github.com/labstack/echo") {
						score += 2
						signals = append(signals, "Echo import in .go files")
						break
					}
				}
			}
		}
		if score >= 4 {
			cands = append(cands, candidate{
				name:     "Echo",
				score:    score,
				language: "Go",
				signals:  signals,
				plan:     echoPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("go.mod") {
			score += 2
			signals = append(signals, "go.mod")
			goMod := strings.ToLower(read("go.mod"))
			if strings.Contains(goMod, "github.com/gofiber/fiber") {
				score += 2
				signals = append(signals, "github.com/gofiber/fiber in go.mod")
			}
		}
		if containsExt(allFiles, ".go") {
			for _, file := range allFiles {
				if strings.HasSuffix(file, ".go") {
					content := strings.ToLower(read(file))
					if strings.Contains(content, "github.com/gofiber/fiber") {
						score += 2
						signals = append(signals, "Fiber import in .go files")
						break
					}
				}
			}
		}
		if score >= 4 {
			cands = append(cands, candidate{
				name:     "Fiber",
				score:    score,
				language: "Go",
				signals:  signals,
				plan:     fiberPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("go.mod") {
			score += 2.5
			signals = append(signals, "go.mod")
		}
		if has("main.go") || containsExt(allFiles, ".go") {
			score += 2
			signals = append(signals, "main.go/.go files")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Go",
				score:    score,
				language: "Go",
				signals:  signals,
				plan:     goPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("app.py") || has("wsgi.py") || has("application.py") {
			score += 2
			signals = append(signals, "Flask app file")
		}
		if has("requirements.txt") || has("Pipfile") || has("pyproject.toml") {
			content := strings.ToLower(read("requirements.txt") + read("Pipfile") + read("pyproject.toml"))
			if strings.Contains(content, "flask") {
				score += 2.5
				signals = append(signals, "Flask in dependencies")
			}
		}
		if has("templates") && dirExists(root, "templates") {
			score += 0.5
			signals = append(signals, "templates/ folder")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Flask",
				score:    score,
				language: "Python",
				signals:  signals,
				plan:     flaskPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"fastify"`) {
				score += 2.5
				signals = append(signals, "package.json has fastify")
			}
		}
		if has("server.js") || has("app.js") {
			score += 1
			signals = append(signals, "server.js or app.js")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Fastify",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     fastifyPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"express"`) {
				score += 2.5
				signals = append(signals, "package.json has express")
			}
			if strings.Contains(pj, `"start"`) && strings.Contains(pj, "node") {
				score += 1
				signals = append(signals, "node start script")
			}
		}
		if has("server.js") || has("app.js") || has("index.js") {
			score += 2
			signals = append(signals, "Express server file")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Express.js",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     expressPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("main.py") || has("app.py") {
			content := strings.ToLower(read("main.py") + read("app.py"))
			if strings.Contains(content, "fastapi") {
				score += 3
				signals = append(signals, "FastAPI import in main/app file")
			}
		}
		if has("requirements.txt") || has("pyproject.toml") {
			content := strings.ToLower(read("requirements.txt") + read("pyproject.toml"))
			if strings.Contains(content, "fastapi") {
				score += 2.5
				signals = append(signals, "FastAPI in dependencies")
			}
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "FastAPI",
				score:    score,
				language: "Python",
				signals:  signals,
				plan:     fastApiPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("pom.xml") {
			content := strings.ToLower(read("pom.xml"))
			if strings.Contains(content, "spring-boot") {
				score += 3
				signals = append(signals, "pom.xml has spring-boot")
			}
		}
		if has("build.gradle") || has("build.gradle.kts") {
			content := strings.ToLower(read("build.gradle") + read("build.gradle.kts"))
			if strings.Contains(content, "spring-boot") {
				score += 3
				signals = append(signals, "gradle has spring-boot")
			}
		}
		if has("src/main/java") && dirExists(root, "src/main/java") {
			score += 1
			signals = append(signals, "Maven/Gradle Java structure")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Spring Boot",
				score:    score,
				language: "Java",
				signals:  signals,
				plan:     springBootPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if containsExt(allFiles, ".csproj") {
			score += 2.5
			signals = append(signals, ".csproj file")
		}
		if has("Program.cs") || has("Startup.cs") {
			score += 2
			signals = append(signals, "ASP.NET Core entry point")
		}
		if has("appsettings.json") {
			score += 1
			signals = append(signals, "appsettings.json")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "ASP.NET Core",
				score:    score,
				language: "C#",
				signals:  signals,
				plan:     aspNetPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("mix.exs") {
			score += 2.5
			signals = append(signals, "mix.exs")
			content := strings.ToLower(read("mix.exs"))
			if strings.Contains(content, "phoenix") {
				score += 2
				signals = append(signals, "Phoenix in mix.exs")
			}
		}
		if dirExists(root, "lib") && dirExists(root, "priv") {
			score += 1
			signals = append(signals, "Elixir project structure")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Phoenix",
				score:    score,
				language: "Elixir",
				signals:  signals,
				plan:     phoenixPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("nest-cli.json") {
			score += 3
			signals = append(signals, "nest-cli.json")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@nestjs/core"`) {
				score += 3
				signals = append(signals, "package.json has @nestjs/core")
			}
		}
		if has("src/main.ts") && has("src/app.module.ts") {
			score += 1
			signals = append(signals, "NestJS app structure")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "NestJS",
				score:    score,
				language: "TypeScript",
				signals:  signals,
				plan:     nestjsPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("remix.config.js") || has("remix.config.ts") {
			score += 3
			signals = append(signals, "remix.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@remix-run/react"`) {
				score += 2.5
				signals = append(signals, "package.json has @remix-run/react")
			}
		}
		if dirExists(root, "app/routes") {
			score += 1
			signals = append(signals, "app/routes/ directory")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Remix",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     remixPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("nuxt.config.js") || has("nuxt.config.ts") {
			score += 3
			signals = append(signals, "nuxt.config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"nuxt"`) {
				score += 2.5
				signals = append(signals, "package.json has nuxt")
			}
		}
		if dirExists(root, "pages") || has("app.vue") {
			score += 1
			signals = append(signals, "pages/ directory or app.vue")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Nuxt.js",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     nuxtPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("symfony.lock") {
			score += 3
			signals = append(signals, "symfony.lock")
		}
		if has("bin/console") {
			score += 2.5
			signals = append(signals, "bin/console")
		}
		if has("composer.json") {
			content := strings.ToLower(read("composer.json"))
			if strings.Contains(content, "symfony") {
				score += 2
				signals = append(signals, "symfony in composer.json")
			}
		}
		if has("config/bundles.php") {
			score += 1
			signals = append(signals, "config/bundles.php")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Symfony",
				score:    score,
				language: "PHP",
				signals:  signals,
				plan:     symfonyPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"fastify"`) {
				score += 2.5
				signals = append(signals, "package.json has fastify")
			}
		}
		if has("server.js") || has("app.js") {
			score += 1
			signals = append(signals, "server.js or app.js")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Fastify",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     fastifyPlan,
			})
		}
	}

	if c := detectGoFramework(root, allFiles, has, read, "Gin", "github.com/gin-gonic/gin", ginPlan); c.score > 0 {
		cands = append(cands, c)
	}

	if c := detectGoFramework(root, allFiles, has, read, "Echo", "github.com/labstack/echo", echoPlan); c.score > 0 {
		cands = append(cands, c)
	}

	if c := detectGoFramework(root, allFiles, has, read, "Fiber", "github.com/gofiber/fiber", fiberPlan); c.score > 0 {
		cands = append(cands, c)
	}

	{
		score := 0.0
		signals := []string{}
		hasContentDir := dirExists(root, "content")
		hasHugoConfig := has("hugo.toml") || has("hugo.yaml") || has("hugo.json")
		hasGenericConfig := has("config.toml") || has("config.yaml")

		if hasHugoConfig {
			score += 3
			signals = append(signals, "hugo config file")
		}
		if hasGenericConfig && !hasHugoConfig && hasContentDir {
			score += 3
			signals = append(signals, "hugo config file")
		}
		if hasContentDir {
			score += 2
			signals = append(signals, "content/ directory")
		}
		if dirExists(root, "themes") {
			score += 1
			signals = append(signals, "themes/ directory")
		}
		if score >= 3 {
			cands = append(cands, candidate{
				name:     "Hugo",
				score:    score,
				language: "Go",
				signals:  signals,
				plan:     hugoPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has(".eleventy.js") || has("eleventy.config.js") {
			score += 3
			signals = append(signals, "eleventy config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@11ty/eleventy"`) {
				score += 2.5
				signals = append(signals, "package.json has @11ty/eleventy")
			}
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Eleventy",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     eleventyPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("_config.yml") {
			score += 3
			signals = append(signals, "_config.yml")
		}
		if has("Gemfile") {
			gemfile := strings.ToLower(read("Gemfile"))
			if strings.Contains(gemfile, "jekyll") {
				score += 2.5
				signals = append(signals, "jekyll in Gemfile")
			}
		}
		if dirExists(root, "_posts") || dirExists(root, "_site") {
			score += 1
			signals = append(signals, "_posts/ or _site/ directory")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Jekyll",
				score:    score,
				language: "Ruby",
				signals:  signals,
				plan:     jekyllPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("docusaurus.config.js") || has("docusaurus.config.ts") {
			score += 3
			signals = append(signals, "docusaurus config")
		}
		if has("package.json") {
			pj := strings.ToLower(read("package.json"))
			if strings.Contains(pj, `"@docusaurus/core"`) {
				score += 2.5
				signals = append(signals, "package.json has @docusaurus/core")
			}
		}
		if dirExists(root, "docs") || dirExists(root, "blog") {
			score += 1
			signals = append(signals, "docs/ or blog/ directory")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Docusaurus",
				score:    score,
				language: "JavaScript/TypeScript",
				signals:  signals,
				plan:     docusaurusPlan,
			})
		}
	}

	// Actix-web (Rust)
	{
		score := 0.0
		signals := []string{}
		if has("Cargo.toml") {
			score += 2.5
			signals = append(signals, "Cargo.toml")
			content := strings.ToLower(read("Cargo.toml"))
			if strings.Contains(content, "actix-web") {
				score += 2.5
				signals = append(signals, "actix-web in Cargo.toml")
			}
		}
		if containsExt(allFiles, ".rs") {
			score += 2
			signals = append(signals, ".rs files")
		}
		if score >= 4 {
			cands = append(cands, candidate{
				name:     "Actix-web",
				score:    score,
				language: "Rust",
				signals:  signals,
				plan:     actixPlan,
			})
		}
	}

	// Axum (Rust)
	{
		score := 0.0
		signals := []string{}
		if has("Cargo.toml") {
			score += 2.5
			signals = append(signals, "Cargo.toml")
			content := strings.ToLower(read("Cargo.toml"))
			if strings.Contains(content, "axum") && strings.Contains(content, "tokio") {
				score += 2.5
				signals = append(signals, "axum and tokio in Cargo.toml")
			}
		}
		if containsExt(allFiles, ".rs") {
			score += 2
			signals = append(signals, ".rs files")
		}
		if score >= 4 {
			cands = append(cands, candidate{
				name:     "Axum",
				score:    score,
				language: "Rust",
				signals:  signals,
				plan:     axumPlan,
			})
		}
	}

	{
		score := 0.0
		signals := []string{}
		if has("Dockerfile") {
			score += 2
			signals = append(signals, "Dockerfile")
		}
		if score > 0 {
			cands = append(cands, candidate{
				name:     "Generic Docker",
				score:    score,
				language: dominantLanguage(extCounts),
				signals:  signals,
				plan:     dockerPlan,
			})
		}
	}

	if len(cands) == 0 {
		lang := dominantLanguage(extCounts)
		meta := map[string]string{"note": "Fell back to generic. Provide custom commands."}

		if runtimeVersion := detectRuntimeVersion(root, lang); runtimeVersion != "" {
			meta["runtime_version"] = runtimeVersion
		}

		monorepoMeta := detectMonorepo(root)
		for k, v := range monorepoMeta {
			meta[k] = v
		}

		out := Detection{
			Framework:   "Unknown",
			Language:    lang,
			Confidence:  0.0,
			Signals:     []string{"no strong framework signals"},
			BuildPlan:   []string{"# Please provide build steps"},
			RunPlan:     []string{"# Please provide run command"},
			Healthcheck: map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30},
			EnvSchema:   []string{},
			Meta:        meta,
		}
		return out
	}

	best := pickBest(cands)

	build, run, health, env, meta := best.plan(root)

	if runtimeVersion := detectRuntimeVersion(root, best.language); runtimeVersion != "" {
		meta["runtime_version"] = runtimeVersion
	}

	monorepoMeta := detectMonorepo(root)
	for k, v := range monorepoMeta {
		meta[k] = v
	}

	out := Detection{
		Framework:   best.name,
		Language:    best.language,
		Confidence:  clamp(best.score/6.0, 0, 1),
		Signals:     best.signals,
		BuildPlan:   build,
		RunPlan:     run,
		Healthcheck: health,
		EnvSchema:   env,
		Meta:        meta,
	}
	return out
}

func DetectAndPrint(root string) {
	detection := DetectFramework(root)
	emitJSON(detection)
}

func pickBest(cands []candidate) candidate {
	best := cands[0]
	for _, c := range cands[1:] {
		if c.score > best.score {
			best = c
			continue
		}
		if c.score == best.score {
			if best.name == "Generic Docker" && c.name != "Generic Docker" {
				best = c
			}
		}
	}
	return best
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func scanTree(root string) ([]string, map[string]int, error) {
	var files []string
	extCounts := map[string]int{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		base := filepath.Base(p)
		if d.IsDir() && (base == ".git" || base == "node_modules" || base == ".venv" || base == "venv" || base == "dist" || base == "build") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			files = append(files, rel)
			ext := strings.ToLower(filepath.Ext(p))
			if ext != "" {
				extCounts[ext]++
			}
		}
		return nil
	})
	return files, extCounts, err
}

func containsExt(files []string, ext string) bool {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ext) {
			return true
		}
	}
	return false
}

func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func dirExists(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, rel))
	return err == nil && fi.IsDir()
}

func dominantLanguage(extCounts map[string]int) string {
	type kv struct {
		ext string
		n   int
	}
	var items []kv
	for k, v := range extCounts {
		items = append(items, kv{k, v})
	}
	langScores := map[string]int{}
	for ext, n := range extCounts {
		lang := mapExt(ext)
		langScores[lang] += n
	}
	bestLang := "Unknown"
	best := 0
	for lang, n := range langScores {
		if n > best {
			best = n
			bestLang = lang
		}
	}
	return bestLang
}

func mapExt(ext string) string {
	switch ext {
	case ".py":
		return "Python"
	case ".js", ".jsx", ".ts", ".tsx":
		return "JavaScript/TypeScript"
	case ".php":
		return "PHP"
	case ".rb":
		return "Ruby"
	case ".go":
		return "Go"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".cs":
		return "C#"
	case ".ex", ".exs":
		return "Elixir"
	case ".vue":
		return "JavaScript/TypeScript"
	case ".svelte":
		return "JavaScript/TypeScript"
	default:
		return "Other"
	}
}

func detectGoFramework(_ string, allFiles []string, has func(string) bool, read func(string) string, frameworkName string, importPath string, planFunc func(string) ([]string, []string, map[string]any, []string, map[string]string)) candidate {
	score := 0.0
	signals := []string{}

	if has("go.mod") {
		content := strings.ToLower(read("go.mod"))
		if strings.Contains(content, importPath) {
			score += 2
			signals = append(signals, fmt.Sprintf("%s in go.mod", importPath))
		}
	}

	if containsExt(allFiles, ".go") {
		for _, f := range allFiles {
			if strings.HasSuffix(f, ".go") {
				content := strings.ToLower(read(f))
				if strings.Contains(content, fmt.Sprintf(`"%s"`, importPath)) {
					score += 2
					signals = append(signals, fmt.Sprintf("%s import in .go files", frameworkName))
					break
				}
			}
		}
	}

	return candidate{
		name:     frameworkName,
		score:    score,
		language: "Go",
		signals:  signals,
		plan:     planFunc,
	}
}

// detectRuntimeVersion detects runtime version from version files
func detectRuntimeVersion(root string, language string) string {
	readFile := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(root, rel))
		return string(b)
	}

	switch language {
	case "JavaScript/TypeScript":
		if fileExists(root, ".nvmrc") {
			return strings.TrimSpace(readFile(".nvmrc"))
		}
		if fileExists(root, ".node-version") {
			return strings.TrimSpace(readFile(".node-version"))
		}
	case "Python":
		if fileExists(root, ".python-version") {
			return strings.TrimSpace(readFile(".python-version"))
		}
		if fileExists(root, "runtime.txt") {
			content := strings.TrimSpace(readFile("runtime.txt"))
			// Parse "python-3.11.0" format
			return strings.TrimPrefix(content, "python-")
		}
	case "Ruby":
		if fileExists(root, ".ruby-version") {
			return strings.TrimSpace(readFile(".ruby-version"))
		}
	case "Go":
		if fileExists(root, ".go-version") {
			return strings.TrimSpace(readFile(".go-version"))
		}
	}
	return ""
}

// detectMonorepo detects monorepo tools and configuration
func detectMonorepo(root string) map[string]string {
	result := map[string]string{}

	readFile := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(root, rel))
		return string(b)
	}

	if fileExists(root, "turbo.json") {
		result["monorepo_tool"] = "turborepo"
	} else if fileExists(root, "nx.json") {
		result["monorepo_tool"] = "nx"
	} else if fileExists(root, "lerna.json") {
		result["monorepo_tool"] = "lerna"
	} else if fileExists(root, "pnpm-workspace.yaml") {
		result["monorepo_tool"] = "pnpm-workspaces"
	} else if fileExists(root, "package.json") {
		content := readFile("package.json")
		if strings.Contains(content, `"workspaces"`) {
			result["monorepo_tool"] = "yarn-workspaces"
		}
	}

	if result["monorepo_tool"] != "" {
		result["monorepo"] = "true"
	}

	return result
}
