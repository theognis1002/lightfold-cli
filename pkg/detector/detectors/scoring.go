package detectors

// Scoring constants for framework detection
// These define the weight of different signals when detecting frameworks

const (
	// ScoreConfigFile represents a framework-specific configuration file
	// Examples: next.config.js, remix.config.js, django settings.py
	// This is the strongest signal that a framework is in use
	ScoreConfigFile = 3.0

	// ScoreDependency represents a framework dependency in package manager files
	// Examples: "next" in package.json, "django" in requirements.txt
	// Strong signal but slightly lower than config files
	ScoreDependency = 2.5

	// ScoreLockfile represents a package manager lockfile
	// Examples: package-lock.json, Gemfile.lock, Cargo.lock
	// Indicates dependencies are installed
	ScoreLockfile = 2.0

	// ScoreBuildTool represents a build tool or CLI
	// Examples: bin/rails, artisan, nest-cli.json
	ScoreBuildTool = 2.5

	// ScoreStructure represents framework-specific directory structure
	// Examples: pages/ folder, app/routes/, src/pages/
	// Weaker signal as directories can be named anything
	ScoreStructure = 1.0

	// ScoreFilePattern represents framework-specific file patterns
	// Examples: .vue files, .astro files
	// Medium strength signal
	ScoreFilePattern = 2.0

	// ScoreScriptPattern represents framework-specific scripts in package.json
	// Examples: "next build", "gatsby develop"
	// Moderate signal strength
	ScoreScriptPattern = 1.0

	// ScoreMinorIndicator represents minor indicators
	// Examples: additional config files, secondary directories
	// Weak signal used for tie-breaking
	ScoreMinorIndicator = 0.5

	// ScoreDockerCompose represents a docker-compose file
	// Highest priority - containerized multi-service setup overrides framework detection
	ScoreDockerCompose = 5.0
)
