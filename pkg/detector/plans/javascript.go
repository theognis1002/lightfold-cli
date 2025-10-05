package plans

import (
	"lightfold/pkg/config"
	"lightfold/pkg/detector/helpers"
	"lightfold/pkg/detector/packagemanagers"
)

// NextPlan returns the build and run plan for Next.js
func NextPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	nextConfig := helpers.ParseNextConfig(root)
	pkg := helpers.ParsePackageJSON(root)

	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}

	var run []string
	switch nextConfig.OutputMode {
	case "standalone":
		run = []string{"node .next/standalone/server.js"}
	case "export":
		run = []string{"# Static export - serve with nginx or CDN"}
	default:
		startScript := helpers.GetProductionStartScript(pkg)
		run = []string{packagemanagers.GetRunCommand(pm, startScript)}
	}

	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NEXT_PUBLIC_*, any server-only envs"}

	meta := map[string]string{
		"package_manager": pm,
		"output_mode":     nextConfig.OutputMode,
	}

	// Only set router if detected
	if nextConfig.Router != "" {
		meta["router"] = nextConfig.Router
	}

	// Set build_output - for backwards compatibility with tests, use .next/ for standalone
	if nextConfig.OutputMode == "standalone" {
		meta["build_output"] = ".next/"
	} else {
		meta["build_output"] = nextConfig.BuildOutput
	}

	// Backwards compatibility: set "export" metadata for static exports
	if nextConfig.OutputMode == "export" {
		meta["export"] = "static"
	}

	if monorepoType := helpers.DetectMonorepoType(root); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}

	return build, run, health, env, meta
}

// RemixPlan returns the build and run plan for Remix
func RemixPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	pkg := helpers.ParsePackageJSON(root)
	adapter := helpers.DetectFrameworkAdapter(pkg, "remix")

	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}

	var run []string
	startScript := helpers.GetProductionStartScript(pkg)

	switch adapter.Type {
	case "node":
		run = []string{packagemanagers.GetRunCommand(pm, startScript)}
	case "deno":
		run = []string{"deno run --allow-net --allow-read --allow-env server.ts"}
	case "cloudflare":
		run = []string{"# Deploy to Cloudflare Workers"}
	default:
		run = []string{packagemanagers.GetRunCommand(pm, startScript)}
	}

	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "SESSION_SECRET"}

	meta := map[string]string{
		"package_manager": pm,
		"build_output":    "build/",
		"adapter":         adapter.Type,
	}

	if monorepoType := helpers.DetectMonorepoType(root); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}

	return build, run, health, env, meta
}

// NuxtPlan returns the build and run plan for Nuxt.js
func NuxtPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}
	run := []string{
		"node .output/server/index.mjs",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NUXT_PUBLIC_*", "NITRO_*"}
	meta := map[string]string{"package_manager": pm, "build_output": ".output/"}
	return build, run, health, env, meta
}

// AstroPlan returns the build and run plan for Astro
func AstroPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	pkg := helpers.ParsePackageJSON(root)
	adapter := helpers.DetectFrameworkAdapter(pkg, "astro")

	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}

	var run []string
	startScript := helpers.GetProductionStartScript(pkg)

	switch adapter.Type {
	case "static":
		run = []string{"# Static site - serve dist/ with nginx or CDN"}
	case "node":
		run = []string{"node dist/server/entry.mjs"}
	case "vercel", "netlify", "cloudflare":
		run = []string{"# Deploy to " + adapter.Type}
	default:
		run = []string{packagemanagers.GetRunCommand(pm, startScript)}
	}

	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PUBLIC_*, any server-only envs for SSR"}

	meta := map[string]string{
		"package_manager": pm,
		"build_output":    "dist/",
		"adapter":         adapter.Type,
		"run_mode":        adapter.RunMode,
	}

	if monorepoType := helpers.DetectMonorepoType(root); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}

	return build, run, health, env, meta
}

// GatsbyPlan returns the build and run plan for Gatsby
func GatsbyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}
	run := []string{
		packagemanagers.GetRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"GATSBY_*, any build-time envs"}
	meta := map[string]string{"package_manager": pm, "build_output": "public/"}
	return build, run, health, env, meta
}

// SveltePlan returns the build and run plan for Svelte/SvelteKit
func SveltePlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	pkg := helpers.ParsePackageJSON(root)
	adapter := helpers.DetectFrameworkAdapter(pkg, "sveltekit")

	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}

	var run []string
	startScript := helpers.GetProductionStartScript(pkg)

	switch adapter.Type {
	case "static":
		run = []string{"# Static site - serve build/ with nginx or CDN"}
	case "node":
		run = []string{"node build"}
	case "vercel", "netlify", "cloudflare":
		run = []string{"# Deploy to " + adapter.Type}
	default:
		run = []string{packagemanagers.GetRunCommand(pm, startScript)}
	}

	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"PUBLIC_*, any server-only envs for SvelteKit SSR"}

	meta := map[string]string{
		"package_manager": pm,
		"build_output":    "build/",
		"adapter":         adapter.Type,
		"run_mode":        adapter.RunMode,
	}

	if monorepoType := helpers.DetectMonorepoType(root); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}

	return build, run, health, env, meta
}

// VuePlan returns the build and run plan for Vue.js
func VuePlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	pkg := helpers.ParsePackageJSON(root)

	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}

	startScript := helpers.GetProductionStartScript(pkg)
	run := []string{packagemanagers.GetRunCommand(pm, startScript)}

	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"VUE_APP_*, VITE_* for Vite-based setups"}

	meta := map[string]string{
		"package_manager": pm,
		"build_output":    "dist/",
	}

	allDeps := mergeDeps(pkg.Dependencies, pkg.DevDeps)
	if vueVersion, exists := allDeps["vue"]; exists {
		if len(vueVersion) > 0 && (vueVersion[0] == '3' || vueVersion[1] == '3') {
			meta["vue_version"] = "3"
		} else if len(vueVersion) > 0 && (vueVersion[0] == '2' || vueVersion[1] == '2') {
			meta["vue_version"] = "2"
		}
	}

	if monorepoType := helpers.DetectMonorepoType(root); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}

	return build, run, health, env, meta
}

// mergeDeps merges dependencies and devDependencies
func mergeDeps(deps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, d := range deps {
		for k, v := range d {
			merged[k] = v
		}
	}
	return merged
}

// AngularPlan returns the build and run plan for Angular
func AngularPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}
	run := []string{
		packagemanagers.GetJSStartCommand(pm),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NG_APP_*, any environment-specific configs"}
	meta := map[string]string{"package_manager": pm, "build_output": "dist/"}
	return build, run, health, env, meta
}

// NestJSPlan returns the build and run plan for NestJS
func NestJSPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}
	run := []string{
		"node dist/main",
		packagemanagers.GetRunCommand(pm, "start:prod"),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

// EleventyPlan returns the build and run plan for Eleventy
func EleventyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetRunCommand(pm, "build"),
	}
	run := []string{
		packagemanagers.GetRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"ELEVENTY_ENV"}
	meta := map[string]string{"package_manager": pm, "build_output": "_site/", "static": "true"}
	return build, run, health, env, meta
}

// DocusaurusPlan returns the build and run plan for Docusaurus
func DocusaurusPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
		packagemanagers.GetJSBuildCommand(pm),
	}
	run := []string{
		packagemanagers.GetRunCommand(pm, "serve"),
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{}
	meta := map[string]string{"package_manager": pm, "build_output": "build/"}
	return build, run, health, env, meta
}

// FastifyPlan returns the build and run plan for Fastify
func FastifyPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	if packagemanagers.DetectDenoRuntime(root) {
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

	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
	}
	run := []string{
		packagemanagers.GetJSStartCommand(pm),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}

// ExpressPlan returns the build and run plan for Express.js
func ExpressPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	if packagemanagers.DetectDenoRuntime(root) {
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

	pm := packagemanagers.DetectJS(root)
	build := []string{
		packagemanagers.GetJSInstallCommand(pm),
	}
	run := []string{
		packagemanagers.GetJSStartCommand(pm),
	}
	health := map[string]any{"path": "/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"NODE_ENV", "PORT", "DATABASE_URL"}
	meta := map[string]string{"package_manager": pm}
	return build, run, health, env, meta
}
