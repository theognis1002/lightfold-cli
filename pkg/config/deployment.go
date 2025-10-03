package config

import (
	"fmt"
	"lightfold/pkg/util"
)

// ProcessDeploymentOptions processes environment variables and build options for a target
// This centralizes the logic that was duplicated across deploy, push, and configure commands
func (t *TargetConfig) ProcessDeploymentOptions(envFile string, envVars []string, skipBuild bool) error {
	// Initialize Deploy options if not present
	if t.Deploy == nil {
		t.Deploy = &DeploymentOptions{
			EnvVars: make(map[string]string),
		}
	}

	// Load environment variables from file if provided
	if envFile != "" {
		envVarsFromFile, err := util.LoadEnvFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}
		for k, v := range envVarsFromFile {
			t.Deploy.EnvVars[k] = v
		}
	}

	// Process individual environment variables
	for _, envVar := range envVars {
		parts := util.SplitEnvVar(envVar)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env var format '%s', expected KEY=VALUE", envVar)
		}
		t.Deploy.EnvVars[parts[0]] = parts[1]
	}

	// Set skip build flag
	t.Deploy.SkipBuild = skipBuild

	return nil
}
