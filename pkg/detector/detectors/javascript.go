package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectJavaScript detects JavaScript/TypeScript frameworks
func DetectJavaScript(root string, allFiles []string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectNextJS(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectRemix(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectNuxt(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectAstro(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectGatsby(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectSvelte(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectVue(root, allFiles, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectAngular(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectNestJS(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectEleventy(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectDocusaurus(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFastify(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectExpress(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectNextJS(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("next.config.js") || h.Has("next.config.ts") {
		score += 2.5
		signals = append(signals, "next.config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"next"`) {
			score += 2.5
			signals = append(signals, "package.json has next")
		}
		if strings.Contains(pj, `"scripts"`) && (strings.Contains(pj, `"next build"`) || strings.Contains(pj, `"next dev"`)) {
			score += 1
			signals = append(signals, "package.json scripts for next")
		}
	}
	if h.DirExists(root, "pages") || h.DirExists(root, "app") {
		score += 0.5
		signals = append(signals, "pages/ or app/ folder")
	}

	return Candidate{
		Name:     "Next.js",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.NextPlan,
	}
}

func detectRemix(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("remix.config.js") || h.Has("remix.config.ts") {
		score += 3
		signals = append(signals, "remix.config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@remix-run/react"`) {
			score += 2.5
			signals = append(signals, "package.json has @remix-run/react")
		}
	}
	if h.DirExists(root, "app/routes") {
		score += 1
		signals = append(signals, "app/routes/ directory")
	}

	return Candidate{
		Name:     "Remix",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.RemixPlan,
	}
}

func detectNuxt(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("nuxt.config.js") || h.Has("nuxt.config.ts") {
		score += 3
		signals = append(signals, "nuxt.config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"nuxt"`) {
			score += 2.5
			signals = append(signals, "package.json has nuxt")
		}
	}
	if h.DirExists(root, "pages") || h.Has("app.vue") {
		score += 1
		signals = append(signals, "pages/ directory or app.vue")
	}

	return Candidate{
		Name:     "Nuxt.js",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.NuxtPlan,
	}
}

func detectAstro(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("astro.config.mjs") || h.Has("astro.config.js") || h.Has("astro.config.ts") {
		score += 3
		signals = append(signals, "astro.config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"astro"`) {
			score += 2.5
			signals = append(signals, "package.json has astro")
		}
		if strings.Contains(pj, `"scripts"`) && (strings.Contains(pj, `"astro build"`) || strings.Contains(pj, `"astro dev"`)) {
			score += 1
			signals = append(signals, "package.json scripts for astro")
		}
	}
	if h.DirExists(root, "src") && h.DirExists(root, "public") {
		score += 0.5
		signals = append(signals, "src/ and public/ folders")
	}

	return Candidate{
		Name:     "Astro",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.AstroPlan,
	}
}

func detectGatsby(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("gatsby-config.js") || h.Has("gatsby-config.ts") {
		score += 3
		signals = append(signals, "gatsby-config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"gatsby"`) {
			score += 2.5
			signals = append(signals, "package.json has gatsby")
		}
		if strings.Contains(pj, `"gatsby build"`) || strings.Contains(pj, `"gatsby develop"`) {
			score += 1
			signals = append(signals, "package.json scripts for gatsby")
		}
	}
	if h.DirExists(root, "src/pages") {
		score += 1
		signals = append(signals, "src/pages/ folder")
	}

	return Candidate{
		Name:     "Gatsby",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.GatsbyPlan,
	}
}

func detectSvelte(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("svelte.config.js") || h.Has("svelte.config.ts") {
		score += 3
		signals = append(signals, "svelte.config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@sveltejs/kit"`) {
			score += 3
			signals = append(signals, "package.json has @sveltejs/kit")
		} else if strings.Contains(pj, `"svelte"`) {
			score += 2.5
			signals = append(signals, "package.json has svelte")
		}
	}
	if h.DirExists(root, "src/routes") {
		score += 1
		signals = append(signals, "src/routes/ folder (SvelteKit)")
	}

	return Candidate{
		Name:     "Svelte",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.SveltePlan,
	}
}

func detectVue(root string, allFiles []string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("vue.config.js") || h.Has("vite.config.js") || h.Has("vite.config.ts") {
		score += 2
		signals = append(signals, "vue/vite config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@vue/cli"`) || strings.Contains(pj, `"nuxt"`) {
			score += 3
			signals = append(signals, "Vue CLI or Nuxt")
		} else if strings.Contains(pj, `"vue"`) {
			score += 2.5
			signals = append(signals, "package.json has vue")
		}
	}
	if h.ContainsExt(allFiles, ".vue") {
		score += 2
		signals = append(signals, ".vue files")
	}

	return Candidate{
		Name:     "Vue.js",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.VuePlan,
	}
}

func detectAngular(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("angular.json") {
		score += 3
		signals = append(signals, "angular.json")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@angular/core"`) {
			score += 3
			signals = append(signals, "package.json has @angular/core")
		}
	}
	if h.Has("tsconfig.json") && (h.DirExists(root, "src/app") || h.Has("src/main.ts")) {
		score += 1
		signals = append(signals, "TypeScript config with Angular structure")
	}

	return Candidate{
		Name:     "Angular",
		Score:    score,
		Language: "TypeScript",
		Signals:  signals,
		Plan:     plans.AngularPlan,
	}
}

func detectNestJS(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("nest-cli.json") {
		score += 3
		signals = append(signals, "nest-cli.json")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@nestjs/core"`) {
			score += 3
			signals = append(signals, "package.json has @nestjs/core")
		}
	}
	if h.Has("src/main.ts") && h.Has("src/app.module.ts") {
		score += 1
		signals = append(signals, "NestJS app structure")
	}

	return Candidate{
		Name:     "NestJS",
		Score:    score,
		Language: "TypeScript",
		Signals:  signals,
		Plan:     plans.NestJSPlan,
	}
}

func detectEleventy(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has(".eleventy.js") || h.Has("eleventy.config.js") {
		score += 3
		signals = append(signals, "eleventy config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@11ty/eleventy"`) {
			score += 2.5
			signals = append(signals, "package.json has @11ty/eleventy")
		}
	}

	return Candidate{
		Name:     "Eleventy",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.EleventyPlan,
	}
}

func detectDocusaurus(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("docusaurus.config.js") || h.Has("docusaurus.config.ts") {
		score += 3
		signals = append(signals, "docusaurus config")
	}
	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"@docusaurus/core"`) {
			score += 2.5
			signals = append(signals, "package.json has @docusaurus/core")
		}
	}
	if h.DirExists(root, "docs") || h.DirExists(root, "blog") {
		score += 1
		signals = append(signals, "docs/ or blog/ directory")
	}

	return Candidate{
		Name:     "Docusaurus",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.DocusaurusPlan,
	}
}

func detectFastify(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"fastify"`) {
			score += 2.5
			signals = append(signals, "package.json has fastify")
		}
	}
	if h.Has("server.js") || h.Has("app.js") {
		score += 1
		signals = append(signals, "server.js or app.js")
	}

	return Candidate{
		Name:     "Fastify",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.FastifyPlan,
	}
}

func detectExpress(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("package.json") {
		pj := strings.ToLower(h.Read("package.json"))
		if strings.Contains(pj, `"express"`) {
			score += 2.5
			signals = append(signals, "package.json has express")
		}
		if strings.Contains(pj, `"start"`) && strings.Contains(pj, "node") {
			score += 1
			signals = append(signals, "node start script")
		}
	}
	if h.Has("server.js") || h.Has("app.js") || h.Has("index.js") {
		score += 2
		signals = append(signals, "Express server file")
	}

	return Candidate{
		Name:     "Express.js",
		Score:    score,
		Language: "JavaScript/TypeScript",
		Signals:  signals,
		Plan:     plans.ExpressPlan,
	}
}
