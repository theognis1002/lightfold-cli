package plans

import (
	"lightfold/pkg/config"
)

// SpringBootPlan returns the build and run plan for Spring Boot
func SpringBootPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	var build []string
	var run []string
	var buildTool string
	var buildOutput string
	if fileExists(root, "pom.xml") {
		build = []string{
			"./mvnw clean package -DskipTests",
		}
		run = []string{
			"java -jar target/*.jar",
		}
		buildTool = "maven"
		buildOutput = "target/"
	} else {
		build = []string{
			"./gradlew build -x test",
		}
		run = []string{
			"java -jar build/libs/*.jar",
		}
		buildTool = "gradle"
		buildOutput = "build/"
	}
	health := map[string]any{"path": "/actuator/health", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"SPRING_PROFILES_ACTIVE", "DATABASE_URL", "SERVER_PORT"}
	meta := map[string]string{"build_tool": buildTool, "build_output": buildOutput}
	return build, run, health, env, meta
}
