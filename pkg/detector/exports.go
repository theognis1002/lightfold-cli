package detector

// Export internal functions for testing

// DetectPackageManager detects JavaScript package manager
func DetectPackageManager(root string) string {
	return detectPackageManager(root)
}

// DetectPythonPackageManager detects Python package manager
func DetectPythonPackageManager(root string) string {
	return detectPythonPackageManager(root)
}

// GetJSInstallCommand gets JS install command for package manager
func GetJSInstallCommand(pm string) string {
	return getJSInstallCommand(pm)
}

// GetJSBuildCommand gets JS build command for package manager
func GetJSBuildCommand(pm string) string {
	return getJSBuildCommand(pm)
}

// GetJSStartCommand gets JS start command for package manager
func GetJSStartCommand(pm string) string {
	return getJSStartCommand(pm)
}

// GetPythonInstallCommand gets Python install command for package manager
func GetPythonInstallCommand(pm string) string {
	return getPythonInstallCommand(pm)
}

// Plan functions for testing

// NextPlan exports nextPlan for testing
func NextPlan(root string) ([]string, []string, map[string]any, []string) {
	return nextPlan(root)
}

// DjangoPlan exports djangoPlan for testing
func DjangoPlan(root string) ([]string, []string, map[string]any, []string) {
	return djangoPlan(root)
}

// GoPlan exports goPlan for testing
func GoPlan(root string) ([]string, []string, map[string]any, []string) {
	return goPlan(root)
}