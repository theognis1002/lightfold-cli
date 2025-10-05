package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectJava detects Java frameworks
func DetectJava(root string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectSpringBoot(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectSpringBoot(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("pom.xml") {
		content := strings.ToLower(h.Read("pom.xml"))
		if strings.Contains(content, "spring-boot") {
			score += 3
			signals = append(signals, "pom.xml has spring-boot")
		}
	}
	if h.Has("build.gradle") || h.Has("build.gradle.kts") {
		content := strings.ToLower(h.Read("build.gradle") + h.Read("build.gradle.kts"))
		if strings.Contains(content, "spring-boot") {
			score += 3
			signals = append(signals, "gradle has spring-boot")
		}
	}
	if h.Has("src/main/java") && h.DirExists(root, "src/main/java") {
		score += 1
		signals = append(signals, "Maven/Gradle Java structure")
	}

	return Candidate{
		Name:     "Spring Boot",
		Score:    score,
		Language: "Java",
		Signals:  signals,
		Plan:     plans.SpringBootPlan,
	}
}
