package runtime

import (
	"fmt"
	"lightfold/pkg/ssh"
	"lightfold/pkg/state"
	"strings"
)

// CleanupUnusedRuntimes removes runtimes that are no longer needed by any apps on the server
// This is called after an app is destroyed to clean up orphaned runtimes
func CleanupUnusedRuntimes(sshExecutor *ssh.Executor, serverIP, destroyedTargetName string) error {
	// 1. Get server state
	serverState, err := state.GetServerState(serverIP)
	if err != nil {
		return fmt.Errorf("failed to get server state: %w", err)
	}

	// 2. Calculate which runtimes are still required by remaining apps
	requiredRuntimes := GetRequiredRuntimesForApps(serverState.DeployedApps)

	// 3. Find unused runtimes: installed - required
	var unusedRuntimes []Runtime
	for _, installedRuntime := range serverState.InstalledRuntimes {
		// Convert state.Runtime to runtime.Runtime
		rt := Runtime(installedRuntime)
		if !requiredRuntimes[rt] {
			unusedRuntimes = append(unusedRuntimes, rt)
		}
	}

	// 4. Nothing to clean up
	if len(unusedRuntimes) == 0 {
		return nil
	}

	// 5. Clean up each unused runtime
	for _, rt := range unusedRuntimes {
		if err := removeRuntime(sshExecutor, rt); err != nil {
			// Log warning but don't fail - cleanup is best-effort
			return fmt.Errorf("failed to remove runtime %s: %w", rt, err)
		}
	}

	// 6. Update server state to remove cleaned-up runtimes
	newRuntimes := []state.Runtime{}
	for _, rt := range serverState.InstalledRuntimes {
		// Convert state.Runtime to runtime.Runtime for checking
		runtimeType := Runtime(rt)
		if requiredRuntimes[runtimeType] {
			newRuntimes = append(newRuntimes, rt)
		}
	}
	serverState.InstalledRuntimes = newRuntimes

	if err := state.SaveServerState(serverState); err != nil {
		return fmt.Errorf("failed to update server state after cleanup: %w", err)
	}

	return nil
}

// GetRequiredRuntimesForApps returns a set of runtimes needed by the given apps
func GetRequiredRuntimesForApps(apps []state.DeployedApp) map[Runtime]bool {
	required := make(map[Runtime]bool)

	for _, app := range apps {
		// Map framework to language to runtime
		language := GetLanguageFromFramework(app.Framework)
		if language != "" {
			runtime := GetRuntimeFromLanguage(language)
			if runtime != RuntimeUnknown {
				required[runtime] = true
			}
		}
	}

	return required
}

// GetLanguageFromFramework maps a framework name to its language
// This is needed because DeployedApp stores Framework, not Language
func GetLanguageFromFramework(framework string) string {
	// JavaScript/TypeScript frameworks
	jsFrameworks := map[string]bool{
		"Next.js":    true,
		"Nuxt.js":    true,
		"Remix":      true,
		"SvelteKit":  true,
		"Astro":      true,
		"Express.js": true,
		"NestJS":     true,
		"Fastify":    true,
	}
	if jsFrameworks[framework] {
		return "JavaScript/TypeScript"
	}

	// Python frameworks
	pythonFrameworks := map[string]bool{
		"Django":  true,
		"FastAPI": true,
		"Flask":   true,
	}
	if pythonFrameworks[framework] {
		return "Python"
	}

	// Go frameworks
	goFrameworks := map[string]bool{
		"Gin":   true,
		"Echo":  true,
		"Fiber": true,
	}
	if goFrameworks[framework] {
		return "Go"
	}

	// PHP frameworks
	phpFrameworks := map[string]bool{
		"Laravel":     true,
		"Symfony":     true,
		"CodeIgniter": true,
	}
	if phpFrameworks[framework] {
		return "PHP"
	}

	// Ruby frameworks
	rubyFrameworks := map[string]bool{
		"Rails":   true,
		"Sinatra": true,
	}
	if rubyFrameworks[framework] {
		return "Ruby"
	}

	// Java frameworks
	javaFrameworks := map[string]bool{
		"Spring Boot": true,
	}
	if javaFrameworks[framework] {
		return "Java"
	}

	// Unknown framework - return empty to skip cleanup
	return ""
}

// removeRuntime performs the actual removal of a runtime from the server
func removeRuntime(sshExecutor *ssh.Executor, rt Runtime) error {
	info := GetRuntimeInfo(rt)

	// Remove APT packages
	if len(info.Packages) > 0 {
		packageList := strings.Join(info.Packages, " ")
		// Use || true to make removal non-fatal
		cmd := fmt.Sprintf("apt-get remove -y %s 2>/dev/null || true", packageList)
		result := sshExecutor.ExecuteSudo(cmd)
		if result.Error != nil {
			// Don't fail on package removal errors - they might not be installed
			// Just continue with directory cleanup
		}
	}

	// Remove directories
	for _, dir := range info.Directories {
		// Use || true to make removal non-fatal
		cmd := fmt.Sprintf("rm -rf %s 2>/dev/null || true", dir)
		result := sshExecutor.ExecuteSudo(cmd)
		if result.Error != nil {
			// Don't fail on directory removal errors
		}
	}

	// Run additional cleanup commands
	for _, cmd := range info.Commands {
		result := sshExecutor.ExecuteSudo(cmd)
		if result.Error != nil {
			// Don't fail on command errors
		}
	}

	// Run apt-get autoremove to clean up unused dependencies
	sshExecutor.ExecuteSudo("apt-get autoremove -y 2>/dev/null || true")

	return nil
}
