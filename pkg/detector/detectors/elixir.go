package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectElixir detects Elixir frameworks
func DetectElixir(fs FSReader) []Candidate {
	var candidates []Candidate

	if c := detectPhoenix(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectPhoenix(fs FSReader) Candidate {
	return NewDetectionBuilder("Phoenix", "Elixir", fs).
		CheckFile("mix.exs", ScoreDependency, "mix.exs").
		CheckContent("mix.exs", "phoenix", ScoreLockfile, "Phoenix in mix.exs").
		CheckCondition(fs.DirExists("lib") && fs.DirExists("priv"), ScoreStructure, "Elixir project structure").
		Build(plans.PhoenixPlan)
}
