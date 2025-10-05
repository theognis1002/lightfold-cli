package detector

import (
	"lightfold/pkg/detector/packagemanagers"
	"lightfold/pkg/detector/plans"
)

// Export internal functions for testing

// DetectPackageManager detects JavaScript package manager
func DetectPackageManager(root string) string {
	return packagemanagers.DetectJS(root)
}

// DetectPythonPackageManager detects Python package manager
func DetectPythonPackageManager(root string) string {
	return packagemanagers.DetectPython(root)
}

// GetJSInstallCommand gets JS install command for package manager
func GetJSInstallCommand(pm string) string {
	return packagemanagers.GetJSInstallCommand(pm)
}

// GetJSBuildCommand gets JS build command for package manager
func GetJSBuildCommand(pm string) string {
	return packagemanagers.GetJSBuildCommand(pm)
}

// GetJSStartCommand gets JS start command for package manager
func GetJSStartCommand(pm string) string {
	return packagemanagers.GetJSStartCommand(pm)
}

// GetPythonInstallCommand gets Python install command for package manager
func GetPythonInstallCommand(pm string) string {
	return packagemanagers.GetPythonInstallCommand(pm)
}

// Plan functions for testing

// NextPlan exports NextPlan for testing
func NextPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	return plans.NextPlan(root)
}

// DjangoPlan exports DjangoPlan for testing
func DjangoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	return plans.DjangoPlan(root)
}

// GoPlan exports GoPlan for testing
func GoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	return plans.GoPlan(root)
}