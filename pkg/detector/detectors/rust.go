package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectRust detects Rust frameworks
func DetectRust(fs FSReader, allFiles []string) []Candidate {
	var candidates []Candidate

	if c := detectActix(fs, allFiles); c.Score >= 4 {
		candidates = append(candidates, c)
	}

	if c := detectAxum(fs, allFiles); c.Score >= 4 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectActix(fs FSReader, allFiles []string) Candidate {
	return NewDetectionBuilder("Actix-web", "Rust", fs).
		CheckFile("Cargo.toml", ScoreDependency, "Cargo.toml").
		CheckContent("Cargo.toml", "actix-web", ScoreDependency, "actix-web in Cargo.toml").
		CheckExtension(allFiles, ".rs", ScoreLockfile, ".rs files").
		Build(plans.ActixPlan)
}

func detectAxum(fs FSReader, allFiles []string) Candidate {
	builder := NewDetectionBuilder("Axum", "Rust", fs).
		CheckFile("Cargo.toml", ScoreDependency, "Cargo.toml").
		CheckExtension(allFiles, ".rs", ScoreLockfile, ".rs files")

	if fs.Has("Cargo.toml") {
		content := strings.ToLower(fs.Read("Cargo.toml"))
		if strings.Contains(content, "axum") && strings.Contains(content, "tokio") {
			builder.score += ScoreDependency
			builder.signals = append(builder.signals, "axum and tokio in Cargo.toml")
		}
	}

	return builder.Build(plans.AxumPlan)
}
