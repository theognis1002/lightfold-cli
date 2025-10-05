package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectRuby detects Ruby frameworks
func DetectRuby(root string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectRails(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectJekyll(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectRails(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("bin/rails") {
		score += 3
		signals = append(signals, "bin/rails")
	}
	if h.Has("Gemfile.lock") {
		score += 2
		signals = append(signals, "Gemfile.lock")
	}
	if h.Has("config/application.rb") {
		score += 0.5
		signals = append(signals, "config/application.rb")
	}

	return Candidate{
		Name:     "Rails",
		Score:    score,
		Language: "Ruby",
		Signals:  signals,
		Plan:     plans.RailsPlan,
	}
}

func detectJekyll(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("_config.yml") {
		score += 3
		signals = append(signals, "_config.yml")
	}
	if h.Has("Gemfile") {
		gemfile := strings.ToLower(h.Read("Gemfile"))
		if strings.Contains(gemfile, "jekyll") {
			score += 2.5
			signals = append(signals, "jekyll in Gemfile")
		}
	}
	if h.DirExists(root, "_posts") || h.DirExists(root, "_site") {
		score += 1
		signals = append(signals, "_posts/ or _site/ directory")
	}

	return Candidate{
		Name:     "Jekyll",
		Score:    score,
		Language: "Ruby",
		Signals:  signals,
		Plan:     plans.JekyllPlan,
	}
}
