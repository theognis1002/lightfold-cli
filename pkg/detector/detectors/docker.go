package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectDocker detects Docker-based projects
func DetectDocker(fs FSReader, dominantLang string) []Candidate {
	var candidates []Candidate

	// Docker Compose - highest priority
	if c := detectDockerCompose(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	// Generic Docker
	if c := detectGenericDocker(fs, dominantLang); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectDockerCompose(fs FSReader) Candidate {
	return NewDetectionBuilder("Docker Compose", "Container", fs).
		CheckAnyFile([]string{
			"docker-compose.yml",
			"docker-compose.yaml",
			"compose.yml",
			"compose.yaml",
		}, ScoreDockerCompose, "docker-compose file").
		Build(plans.DockerComposePlan)
}

func detectGenericDocker(fs FSReader, dominantLang string) Candidate {
	return NewDetectionBuilder("Generic Docker", dominantLang, fs).
		CheckFile("Dockerfile", ScoreLockfile, "Dockerfile").
		Build(plans.DockerPlan)
}
