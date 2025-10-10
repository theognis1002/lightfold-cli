package utils

import (
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/util"
	"os"
)

// LoadConfigOrExit loads the config or exits on error
func LoadConfigOrExit() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// LoadTargetOrExit loads a target by name or exits on error
func LoadTargetOrExit(cfg *config.Config, targetName string) config.TargetConfig {
	target, exists := cfg.GetTarget(targetName)
	if !exists {
		fmt.Fprintf(os.Stderr, "Error: Target '%s' not found\n", targetName)
		fmt.Fprintf(os.Stderr, "\nRun 'lightfold status' to list all configured targets\n")
		os.Exit(1)
	}
	return target
}

// ResolveTarget resolves a target from config by name or path
// Returns an error instead of calling os.Exit to make it testable
func ResolveTarget(cfg *config.Config, targetFlag string, pathArg string) (config.TargetConfig, string, error) {
	effectiveTarget := targetFlag
	if effectiveTarget == "" {
		if pathArg != "" {
			effectiveTarget = pathArg
		} else {
			effectiveTarget = "."
		}
	}

	if target, exists := cfg.GetTarget(effectiveTarget); exists {
		return target, effectiveTarget, nil
	}

	projectPath, err := util.ValidateProjectPath(effectiveTarget)
	if err != nil {
		return config.TargetConfig{}, "", err
	}

	targetName, target, exists := cfg.FindTargetByPath(projectPath)
	if !exists {
		targetName = util.GetTargetName(projectPath)
		target, exists = cfg.GetTarget(targetName)
		if !exists {
			return config.TargetConfig{}, "", fmt.Errorf("no target found for this project\nRun 'lightfold create' first")
		}
	}

	return target, targetName, nil
}

// ResolveTargetOrExit resolves a target and exits on error (convenience wrapper)
func ResolveTargetOrExit(cfg *config.Config, targetFlag string, pathArg string) (config.TargetConfig, string) {
	target, targetName, err := ResolveTarget(cfg, targetFlag, pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return target, targetName
}
