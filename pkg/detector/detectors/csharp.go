package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectCSharp detects C# frameworks
func DetectCSharp(root string, allFiles []string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectASPNET(root, allFiles, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectASPNET(root string, allFiles []string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.ContainsExt(allFiles, ".csproj") {
		score += 2.5
		signals = append(signals, ".csproj file")
	}
	if h.Has("Program.cs") || h.Has("Startup.cs") {
		score += 2
		signals = append(signals, "ASP.NET Core entry point")
	}
	if h.Has("appsettings.json") {
		score += 1
		signals = append(signals, "appsettings.json")
	}

	return Candidate{
		Name:     "ASP.NET Core",
		Score:    score,
		Language: "C#",
		Signals:  signals,
		Plan:     plans.AspNetPlan,
	}
}
