package detectors

import (
	"fmt"
	"lightfold/pkg/detector/plans"
)

// DetectGo detects Go frameworks
func DetectGo(fs FSReader, allFiles []string) []Candidate {
	var candidates []Candidate
	hasSpecificFramework := false

	if c := detectGoFramework(fs, allFiles, "Gin", "github.com/gin-gonic/gin", plans.GinPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectGoFramework(fs, allFiles, "Echo", "github.com/labstack/echo", plans.EchoPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectGoFramework(fs, allFiles, "Fiber", "github.com/gofiber/fiber", plans.FiberPlan); c.Score >= 4 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if c := detectHugo(fs); c.Score >= 3 {
		candidates = append(candidates, c)
		hasSpecificFramework = true
	}

	if !hasSpecificFramework {
		if c := detectGenericGo(fs, allFiles); c.Score > 0 {
			candidates = append(candidates, c)
		}
	}

	return candidates
}

func detectGoFramework(fs FSReader, allFiles []string, frameworkName string, importPath string, planFunc any) Candidate {
	return NewDetectionBuilder(frameworkName, "Go", fs).
		CheckContent("go.mod", importPath, ScoreLockfile, fmt.Sprintf("%s in go.mod", importPath)).
		CheckContentInFiles(allFiles, ".go", `"`+importPath, ScoreLockfile, fmt.Sprintf("%s import in .go files", frameworkName)).
		Build(planFunc)
}

func detectHugo(fs FSReader) Candidate {
	builder := NewDetectionBuilder("Hugo", "Go", fs).
		CheckAnyPath([]string{"hugo.toml", "hugo.yaml", "hugo.json"}, ScoreConfigFile, "hugo config file")

	// Check for generic config files only if no Hugo-specific config exists
	hasHugoConfig := fs.Has("hugo.toml") || fs.Has("hugo.yaml") || fs.Has("hugo.json")
	if !hasHugoConfig {
		hasContentDir := fs.DirExists("content")
		hasGenericConfig := fs.Has("config.toml") || fs.Has("config.yaml")
		builder = builder.CheckCondition(hasGenericConfig && hasContentDir, ScoreConfigFile, "hugo config file")
	}

	return builder.
		CheckDir("content", ScoreLockfile, "content/ directory").
		CheckDir("themes", ScoreStructure, "themes/ directory").
		Build(plans.HugoPlan)
}

func detectGenericGo(fs FSReader, allFiles []string) Candidate {
	return NewDetectionBuilder("Go", "Go", fs).
		CheckFile("go.mod", ScoreDependency, "go.mod").
		CheckCondition(fs.Has("main.go") || fs.ContainsExt(allFiles, ".go"), ScoreLockfile, "main.go/.go files").
		Build(plans.GoPlan)
}
