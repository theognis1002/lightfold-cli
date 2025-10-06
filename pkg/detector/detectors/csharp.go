package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectCSharp detects C# frameworks
func DetectCSharp(fs FSReader, allFiles []string) []Candidate {
	var candidates []Candidate

	if c := detectASPNET(fs, allFiles); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectASPNET(fs FSReader, allFiles []string) Candidate {
	return NewDetectionBuilder("ASP.NET Core", "C#", fs).
		CheckExtension(allFiles, ".csproj", ScoreDependency, ".csproj file").
		CheckCondition(fs.Has("Program.cs") || fs.Has("Startup.cs"), ScoreLockfile, "ASP.NET Core entry point").
		CheckFile("appsettings.json", ScoreStructure, "appsettings.json").
		Build(plans.AspNetPlan)
}
