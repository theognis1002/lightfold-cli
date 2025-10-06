package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectJava detects Java frameworks
func DetectJava(fs FSReader) []Candidate {
	var candidates []Candidate

	if c := detectSpringBoot(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectSpringBoot(fs FSReader) Candidate {
	return NewDetectionBuilder("Spring Boot", "Java", fs).
		CheckContent("pom.xml", "spring-boot", ScoreConfigFile, "pom.xml has spring-boot").
		CheckMultipleContent([]string{"build.gradle", "build.gradle.kts"}, "spring-boot", ScoreConfigFile, "gradle has spring-boot").
		CheckCondition(fs.Has("src/main/java") && fs.DirExists("src/main/java"), ScoreStructure, "Maven/Gradle Java structure").
		Build(plans.SpringBootPlan)
}
