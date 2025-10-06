package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectPHP detects PHP frameworks
func DetectPHP(fs FSReader) []Candidate {
	var candidates []Candidate

	if c := detectLaravel(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectSymfony(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectLaravel(fs FSReader) Candidate {
	return NewDetectionBuilder("Laravel", "PHP", fs).
		CheckFile("artisan", ScoreConfigFile, "artisan").
		CheckFile("composer.lock", ScoreLockfile, "composer.lock").
		CheckFile("config/app.php", ScoreMinorIndicator, "config/app.php").
		Build(plans.LaravelPlan)
}

func detectSymfony(fs FSReader) Candidate {
	return NewDetectionBuilder("Symfony", "PHP", fs).
		CheckFile("symfony.lock", ScoreConfigFile, "symfony.lock").
		CheckFile("bin/console", ScoreBuildTool, "bin/console").
		CheckDependency("composer.json", "symfony", ScoreLockfile, "symfony in composer.json").
		CheckFile("config/bundles.php", ScoreStructure, "config/bundles.php").
		Build(plans.SymfonyPlan)
}
