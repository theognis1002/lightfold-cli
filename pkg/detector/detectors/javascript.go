package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectJavaScript detects JavaScript/TypeScript frameworks
func DetectJavaScript(fs FSReader, allFiles []string) []Candidate {
	var candidates []Candidate

	if c := detectNextJS(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectRemix(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectNuxt(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectAstro(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectGatsby(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectSvelte(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectVue(fs, allFiles); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectAngular(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectNestJS(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectTRPC(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectEleventy(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectDocusaurus(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFastify(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectExpress(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectNextJS(fs FSReader) Candidate {
	return NewDetectionBuilder("Next.js", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"next.config.js", "next.config.ts"}, ScoreBuildTool, "next.config").
		CheckDependency("package.json", `"next"`, ScoreDependency, "package.json has next").
		CheckContent("package.json", `"next build"`, ScoreScriptPattern, "package.json scripts for next").
		CheckAnyDir([]string{"pages", "app"}, ScoreMinorIndicator, "pages/ or app/ folder").
		Build(plans.NextPlan)
}

func detectRemix(fs FSReader) Candidate {
	return NewDetectionBuilder("Remix", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"remix.config.js", "remix.config.ts"}, ScoreConfigFile, "remix.config").
		CheckDependency("package.json", `"@remix-run/react"`, ScoreDependency, "package.json has @remix-run/react").
		CheckDir("app/routes", ScoreStructure, "app/routes/ directory").
		Build(plans.RemixPlan)
}

func detectNuxt(fs FSReader) Candidate {
	return NewDetectionBuilder("Nuxt.js", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"nuxt.config.js", "nuxt.config.ts"}, ScoreConfigFile, "nuxt.config").
		CheckDependency("package.json", `"nuxt"`, ScoreDependency, "package.json has nuxt").
		CheckCondition(fs.DirExists("pages") || fs.Has("app.vue"), ScoreStructure, "pages/ directory or app.vue").
		Build(plans.NuxtPlan)
}

func detectAstro(fs FSReader) Candidate {
	return NewDetectionBuilder("Astro", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"astro.config.mjs", "astro.config.js", "astro.config.ts"}, ScoreConfigFile, "astro.config").
		CheckDependency("package.json", `"astro"`, ScoreDependency, "package.json has astro").
		CheckContent("package.json", `"astro build"`, ScoreScriptPattern, "package.json scripts for astro").
		CheckCondition(fs.DirExists("src") && fs.DirExists("public"), ScoreMinorIndicator, "src/ and public/ folders").
		Build(plans.AstroPlan)
}

func detectGatsby(fs FSReader) Candidate {
	return NewDetectionBuilder("Gatsby", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"gatsby-config.js", "gatsby-config.ts"}, ScoreConfigFile, "gatsby-config").
		CheckDependency("package.json", `"gatsby"`, ScoreDependency, "package.json has gatsby").
		CheckContent("package.json", `"gatsby build"`, ScoreScriptPattern, "package.json scripts for gatsby").
		CheckDir("src/pages", ScoreStructure, "src/pages/ folder").
		Build(plans.GatsbyPlan)
}

func detectSvelte(fs FSReader) Candidate {
	return NewDetectionBuilder("Svelte", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"svelte.config.js", "svelte.config.ts"}, ScoreConfigFile, "svelte.config").
		CheckDependencyPriority("package.json", []DependencyCheck{
			{Dependency: `"@sveltejs/kit"`, Score: ScoreConfigFile, Signal: "package.json has @sveltejs/kit"},
			{Dependency: `"svelte"`, Score: ScoreDependency, Signal: "package.json has svelte"},
		}).
		CheckDir("src/routes", ScoreStructure, "src/routes/ folder (SvelteKit)").
		Build(plans.SveltePlan)
}

func detectVue(fs FSReader, allFiles []string) Candidate {
	return NewDetectionBuilder("Vue.js", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"vue.config.js", "vite.config.js", "vite.config.ts"}, ScoreLockfile, "vue/vite config").
		CheckDependencyPriority("package.json", []DependencyCheck{
			{Dependency: `"@vue/cli"`, Score: ScoreConfigFile, Signal: "Vue CLI"},
			{Dependency: `"nuxt"`, Score: ScoreConfigFile, Signal: "Nuxt"},
			{Dependency: `"vue"`, Score: ScoreDependency, Signal: "package.json has vue"},
		}).
		CheckExtension(allFiles, ".vue", ScoreFilePattern, ".vue files").
		Build(plans.VuePlan)
}

func detectAngular(fs FSReader) Candidate {
	return NewDetectionBuilder("Angular", "TypeScript", fs).
		CheckFile("angular.json", ScoreConfigFile, "angular.json").
		CheckDependency("package.json", `"@angular/core"`, ScoreConfigFile, "package.json has @angular/core").
		CheckCondition(fs.Has("tsconfig.json") && (fs.DirExists("src/app") || fs.Has("src/main.ts")),
			ScoreStructure, "TypeScript config with Angular structure").
		Build(plans.AngularPlan)
}

func detectNestJS(fs FSReader) Candidate {
	return NewDetectionBuilder("NestJS", "TypeScript", fs).
		CheckFile("nest-cli.json", ScoreConfigFile, "nest-cli.json").
		CheckDependency("package.json", `"@nestjs/core"`, ScoreConfigFile, "package.json has @nestjs/core").
		CheckCondition(fs.Has("src/main.ts") && fs.Has("src/app.module.ts"), ScoreStructure, "NestJS app structure").
		Build(plans.NestJSPlan)
}

func detectTRPC(fs FSReader) Candidate {
	return NewDetectionBuilder("tRPC", "TypeScript", fs).
		CheckDependency("package.json", `"@trpc/server"`, ScoreConfigFile, "package.json has @trpc/server").
		CheckDependency("package.json", `"@trpc/client"`, ScoreLockfile, "package.json has @trpc/client").
		CheckDependency("package.json", `"zod"`, ScoreStructure, "zod validation").
		CheckDependency("package.json", `"@trpc/server/adapters/standalone"`, ScoreMinorIndicator, "standalone adapter").
		CheckAnyPath([]string{"server/routers", "src/server/routers"}, ScoreBuildTool, "tRPC router directory").
		CheckAnyPath([]string{"server/trpc.ts", "src/server/trpc.ts", "server/router.ts", "src/server/router.ts"}, ScoreBuildTool, "tRPC router files").
		CheckFile("tsconfig.json", ScoreMinorIndicator, "TypeScript config").
		Build(plans.TRPCPlan)
}

func detectEleventy(fs FSReader) Candidate {
	return NewDetectionBuilder("Eleventy", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{".eleventy.js", "eleventy.config.js"}, ScoreConfigFile, "eleventy config").
		CheckDependency("package.json", `"@11ty/eleventy"`, ScoreDependency, "package.json has @11ty/eleventy").
		Build(plans.EleventyPlan)
}

func detectDocusaurus(fs FSReader) Candidate {
	return NewDetectionBuilder("Docusaurus", "JavaScript/TypeScript", fs).
		CheckAnyFile([]string{"docusaurus.config.js", "docusaurus.config.ts"}, ScoreConfigFile, "docusaurus config").
		CheckDependency("package.json", `"@docusaurus/core"`, ScoreDependency, "package.json has @docusaurus/core").
		CheckAnyDir([]string{"docs", "blog"}, ScoreStructure, "docs/ or blog/ directory").
		Build(plans.DocusaurusPlan)
}

func detectFastify(fs FSReader) Candidate {
	return NewDetectionBuilder("Fastify", "JavaScript/TypeScript", fs).
		CheckDependency("package.json", `"fastify"`, ScoreDependency, "package.json has fastify").
		CheckAnyFile([]string{"server.js", "app.js"}, ScoreStructure, "server.js or app.js").
		Build(plans.FastifyPlan)
}

func detectExpress(fs FSReader) Candidate {
	return NewDetectionBuilder("Express.js", "JavaScript/TypeScript", fs).
		CheckDependency("package.json", `"express"`, ScoreDependency, "package.json has express").
		CheckContent("package.json", `"start"`, ScoreStructure, "node start script").
		CheckAnyFile([]string{"server.js", "app.js", "index.js"}, ScoreLockfile, "Express server file").
		Build(plans.ExpressPlan)
}
