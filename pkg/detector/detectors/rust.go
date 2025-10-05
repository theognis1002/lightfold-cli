package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectRust detects Rust frameworks
func DetectRust(root string, allFiles []string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectActix(root, allFiles, helpers); c.Score >= 4 {
		candidates = append(candidates, c)
	}

	if c := detectAxum(root, allFiles, helpers); c.Score >= 4 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectActix(root string, allFiles []string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("Cargo.toml") {
		score += 2.5
		signals = append(signals, "Cargo.toml")
		content := strings.ToLower(h.Read("Cargo.toml"))
		if strings.Contains(content, "actix-web") {
			score += 2.5
			signals = append(signals, "actix-web in Cargo.toml")
		}
	}
	if h.ContainsExt(allFiles, ".rs") {
		score += 2
		signals = append(signals, ".rs files")
	}

	return Candidate{
		Name:     "Actix-web",
		Score:    score,
		Language: "Rust",
		Signals:  signals,
		Plan:     plans.ActixPlan,
	}
}

func detectAxum(root string, allFiles []string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("Cargo.toml") {
		score += 2.5
		signals = append(signals, "Cargo.toml")
		content := strings.ToLower(h.Read("Cargo.toml"))
		if strings.Contains(content, "axum") && strings.Contains(content, "tokio") {
			score += 2.5
			signals = append(signals, "axum and tokio in Cargo.toml")
		}
	}
	if h.ContainsExt(allFiles, ".rs") {
		score += 2
		signals = append(signals, ".rs files")
	}

	return Candidate{
		Name:     "Axum",
		Score:    score,
		Language: "Rust",
		Signals:  signals,
		Plan:     plans.AxumPlan,
	}
}
