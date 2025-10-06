package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectRuby detects Ruby frameworks
func DetectRuby(fs FSReader) []Candidate {
	var candidates []Candidate

	if c := detectRails(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectJekyll(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectRails(fs FSReader) Candidate {
	return NewDetectionBuilder("Rails", "Ruby", fs).
		CheckFile("bin/rails", ScoreConfigFile, "bin/rails").
		CheckFile("Gemfile.lock", ScoreLockfile, "Gemfile.lock").
		CheckFile("config/application.rb", ScoreMinorIndicator, "config/application.rb").
		Build(plans.RailsPlan)
}

func detectJekyll(fs FSReader) Candidate {
	return NewDetectionBuilder("Jekyll", "Ruby", fs).
		CheckFile("_config.yml", ScoreConfigFile, "_config.yml").
		CheckDependency("Gemfile", "jekyll", ScoreDependency, "jekyll in Gemfile").
		CheckAnyDir([]string{"_posts", "_site"}, ScoreStructure, "_posts/ or _site/ directory").
		Build(plans.JekyllPlan)
}
