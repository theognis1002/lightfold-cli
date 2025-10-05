package packagemanagers

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectPython detects the Python package manager used in a project
func DetectPython(root string) string {
	fileExists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}

	switch {
	case fileExists("uv.lock"):
		return "uv"
	case fileExists("pdm.lock"):
		return "pdm"
	case fileExists("poetry.lock"):
		return "poetry"
	case fileExists("Pipfile.lock"):
		return "pipenv"
	default:
		return "pip"
	}
}

// GetPythonInstallCommand returns the install command for the given package manager
func GetPythonInstallCommand(pm string) string {
	switch pm {
	case "uv":
		return "uv sync"
	case "pdm":
		return "pdm install --prod"
	case "poetry":
		return "poetry install"
	case "pipenv":
		return "pipenv install"
	default:
		return "pip install -r requirements.txt"
	}
}

// DetectDjangoServerType detects whether Django project uses ASGI or WSGI
func DetectDjangoServerType(root string) string {
	fileExists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}

	readFile := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(root, rel))
		return string(b)
	}

	// Priority 1: Check for asgi.py file
	if fileExists("asgi.py") {
		return "asgi"
	}

	// Priority 2: Check common Django project structures for asgi.py
	// Look for */asgi.py in common locations
	commonPaths := []string{
		"config/asgi.py",
		"core/asgi.py",
		"mysite/asgi.py",
		"project/asgi.py",
	}
	for _, path := range commonPaths {
		if fileExists(path) {
			return "asgi"
		}
	}

	// Priority 3: Check settings.py for ASGI_APPLICATION
	settingsPaths := []string{
		"settings.py",
		"settings/base.py",
		"config/settings.py",
		"core/settings.py",
	}
	for _, settingsPath := range settingsPaths {
		if fileExists(settingsPath) {
			content := readFile(settingsPath)
			if len(content) > 0 && (
				filepath.Base(settingsPath) == "settings.py" || filepath.Base(settingsPath) == "base.py") {
				// Check if ASGI_APPLICATION is defined
				if strings.Contains(content, "ASGI_APPLICATION") ||
					strings.Contains(content, "asgi") {
					return "asgi"
				}
			}
		}
	}

	// Priority 4: Check dependencies for ASGI servers (uvicorn, daphne, channels)
	depFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}
	for _, depFile := range depFiles {
		if fileExists(depFile) {
			content := readFile(depFile)
			if strings.Contains(content, "uvicorn") ||
				strings.Contains(content, "daphne") ||
				strings.Contains(content, "channels") {
				return "asgi"
			}
		}
	}

	// Priority 5: Check for wsgi.py (traditional Django)
	if fileExists("wsgi.py") {
		return "wsgi"
	}

	// Check common Django project structures for wsgi.py
	for _, path := range []string{"config/wsgi.py", "core/wsgi.py", "mysite/wsgi.py", "project/wsgi.py"} {
		if fileExists(path) {
			return "wsgi"
		}
	}

	// Default to WSGI (safest fallback for Django)
	return "wsgi"
}

// GetDjangoRunCommand returns the appropriate run command based on server type
func GetDjangoRunCommand(serverType, projectName string) string {
	if serverType == "asgi" {
		if projectName != "" {
			return "uvicorn " + projectName + ".asgi:application --host 0.0.0.0 --port 8000"
		}
		return "uvicorn asgi:application --host 0.0.0.0 --port 8000"
	}

	// WSGI (default)
	if projectName != "" {
		return "gunicorn " + projectName + ".wsgi:application --bind 0.0.0.0:8000 --workers 2"
	}
	return "gunicorn <yourproject>.wsgi:application --bind 0.0.0.0:8000 --workers 2"
}
