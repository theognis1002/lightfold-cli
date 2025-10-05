package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectPHP detects PHP frameworks
func DetectPHP(root string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectLaravel(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectSymfony(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectLaravel(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("artisan") {
		score += 3
		signals = append(signals, "artisan")
	}
	if h.Has("composer.lock") {
		score += 2
		signals = append(signals, "composer.lock")
	}
	if h.Has("config/app.php") {
		score += 0.5
		signals = append(signals, "config/app.php")
	}

	return Candidate{
		Name:     "Laravel",
		Score:    score,
		Language: "PHP",
		Signals:  signals,
		Plan:     plans.LaravelPlan,
	}
}

func detectSymfony(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("symfony.lock") {
		score += 3
		signals = append(signals, "symfony.lock")
	}
	if h.Has("bin/console") {
		score += 2.5
		signals = append(signals, "bin/console")
	}
	if h.Has("composer.json") {
		composerContent := strings.ToLower(h.Read("composer.json"))
		if strings.Contains(composerContent, "symfony") {
			score += 2
			signals = append(signals, "symfony in composer.json")
		}
	}
	if h.Has("config/bundles.php") {
		score += 1
		signals = append(signals, "config/bundles.php")
	}

	return Candidate{
		Name:     "Symfony",
		Score:    score,
		Language: "PHP",
		Signals:  signals,
		Plan:     plans.SymfonyPlan,
	}
}
