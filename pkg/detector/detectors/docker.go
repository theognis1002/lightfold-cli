package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectDocker detects Docker-based projects
func DetectDocker(root string, dominantLang string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectGenericDocker(root, dominantLang, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectGenericDocker(root string, dominantLang string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("Dockerfile") {
		score += 2
		signals = append(signals, "Dockerfile")
	}

	return Candidate{
		Name:     "Generic Docker",
		Score:    score,
		Language: dominantLang,
		Signals:  signals,
		Plan:     plans.DockerPlan,
	}
}
