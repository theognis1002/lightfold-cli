package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectElixir detects Elixir frameworks
func DetectElixir(root string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectPhoenix(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectPhoenix(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("mix.exs") {
		score += 2.5
		signals = append(signals, "mix.exs")
		content := strings.ToLower(h.Read("mix.exs"))
		if strings.Contains(content, "phoenix") {
			score += 2
			signals = append(signals, "Phoenix in mix.exs")
		}
	}
	if h.DirExists(root, "lib") && h.DirExists(root, "priv") {
		score += 1
		signals = append(signals, "Elixir project structure")
	}

	return Candidate{
		Name:     "Phoenix",
		Score:    score,
		Language: "Elixir",
		Signals:  signals,
		Plan:     plans.PhoenixPlan,
	}
}
