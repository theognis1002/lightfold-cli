package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectDocker detects Docker-based projects
func DetectDocker(fs FSReader, dominantLang string) []Candidate {
	var candidates []Candidate

	if c := detectGenericDocker(fs, dominantLang); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectGenericDocker(fs FSReader, dominantLang string) Candidate {
	return NewDetectionBuilder("Generic Docker", dominantLang, fs).
		CheckFile("Dockerfile", ScoreLockfile, "Dockerfile").
		Build(plans.DockerPlan)
}
