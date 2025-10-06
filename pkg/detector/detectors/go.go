package detectors

import (
	"fmt"
	"strings"

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
	builder := NewDetectionBuilder(frameworkName, "Go", fs).
		CheckContent("go.mod", importPath, ScoreLockfile, fmt.Sprintf("%s in go.mod", importPath))

	if fs.ContainsExt(allFiles, ".go") {
		for _, f := range allFiles {
			if strings.HasSuffix(f, ".go") {
				content := strings.ToLower(fs.Read(f))
				if strings.Contains(content, `"`+importPath) {
					builder.score += ScoreLockfile
					builder.signals = append(builder.signals, fmt.Sprintf("%s import in .go files", frameworkName))
					break
				}
			}
		}
	}

	return builder.Build(planFunc)
}

func detectHugo(fs FSReader) Candidate {
	hasContentDir := fs.DirExists("content")
	hasHugoConfig := fs.Has("hugo.toml") || fs.Has("hugo.yaml") || fs.Has("hugo.json")
	hasGenericConfig := fs.Has("config.toml") || fs.Has("config.yaml")

	builder := NewDetectionBuilder("Hugo", "Go", fs)

	if hasHugoConfig {
		builder.score += ScoreConfigFile
		builder.signals = append(builder.signals, "hugo config file")
	}
	if hasGenericConfig && !hasHugoConfig && hasContentDir {
		builder.score += ScoreConfigFile
		builder.signals = append(builder.signals, "hugo config file")
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
