package detectors

import (
	"fmt"
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectGo detects Go frameworks
func DetectGo(root string, allFiles []string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate
	hasSpecificFramework := false

	if c := detectGoFramework(allFiles, helpers, "Gin", "github.com/gin-gonic/gin", plans.GinPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectGoFramework(allFiles, helpers, "Echo", "github.com/labstack/echo", plans.EchoPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectGoFramework(allFiles, helpers, "Fiber", "github.com/gofiber/fiber", plans.FiberPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectHugo(root, helpers); c.Score >= 3 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if !hasSpecificFramework {
		if c := detectGenericGo(root, allFiles, helpers); c.Score > 0 {
			candidates = append(candidates, c)
		}
	}

	return candidates
}

func detectGoFramework(allFiles []string, h HelperFuncs, frameworkName string, importPath string, planFunc func(string) ([]string, []string, map[string]any, []string, map[string]string)) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("go.mod") {
		content := strings.ToLower(h.Read("go.mod"))
		if strings.Contains(content, importPath) {
			score += 2
			signals = append(signals, fmt.Sprintf("%s in go.mod", importPath))
		}
	}

	if h.ContainsExt(allFiles, ".go") {
		for _, f := range allFiles {
			if strings.HasSuffix(f, ".go") {
				content := strings.ToLower(h.Read(f))
				if strings.Contains(content, `"`+importPath) {
					score += 2
					signals = append(signals, fmt.Sprintf("%s import in .go files", frameworkName))
					break
				}
			}
		}
	}

	return Candidate{
		Name:     frameworkName,
		Score:    score,
		Language: "Go",
		Signals:  signals,
		Plan:     planFunc,
	}
}

func detectHugo(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}
	hasContentDir := h.DirExists(root, "content")
	hasHugoConfig := h.Has("hugo.toml") || h.Has("hugo.yaml") || h.Has("hugo.json")
	hasGenericConfig := h.Has("config.toml") || h.Has("config.yaml")

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
	if h.DirExists(root, "themes") {
		score += 1
		signals = append(signals, "themes/ directory")
	}

	return Candidate{
		Name:     "Hugo",
		Score:    score,
		Language: "Go",
		Signals:  signals,
		Plan:     plans.HugoPlan,
	}
}

func detectGenericGo(root string, allFiles []string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("go.mod") {
		score += 2.5
		signals = append(signals, "go.mod")
	}
	if h.Has("main.go") || h.ContainsExt(allFiles, ".go") {
		score += 2
		signals = append(signals, "main.go/.go files")
	}

	return Candidate{
		Name:     "Go",
		Score:    score,
		Language: "Go",
		Signals:  signals,
		Plan:     plans.GoPlan,
	}
}
