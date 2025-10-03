package cmd

import (
	"fmt"
	"lightfold/pkg/config"
	"os"
)

// loadConfigOrExit loads the configuration and exits with an error message if it fails
func loadConfigOrExit() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// loadTargetOrExit loads a specific target configuration by name and exits if not found
func loadTargetOrExit(cfg *config.Config, targetName string) config.TargetConfig {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: Target '%s' not found\n", targetName)
		fmt.Fprintf(os.Stderr, "\nRun 'lightfold status' to list all configured targets\n")
		os.Exit(1)
	}
	return target
}
