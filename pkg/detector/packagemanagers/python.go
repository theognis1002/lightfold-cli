package packagemanagers

import (
	"path/filepath"
	"strings"
)

// DetectPython detects the Python package manager used in a project
func DetectPython(fs FSReader) string {
	switch {
	case fs.Has("uv.lock"):
		return "uv"
	case fs.Has("pdm.lock"):
		return "pdm"
	case fs.Has("poetry.lock"):
		return "poetry"
	case fs.Has("Pipfile.lock"):
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

func DetectDjangoServerType(fs FSReader) string {
	if fs.Has("asgi.py") {
		return "asgi"
	}

	commonPaths := []string{
		"config/asgi.py",
		"core/asgi.py",
		"mysite/asgi.py",
		"project/asgi.py",
	}
	for _, path := range commonPaths {
		if fs.Has(path) {
			return "asgi"
		}
	}

	settingsPaths := []string{
		"settings.py",
		"settings/base.py",
		"config/settings.py",
		"core/settings.py",
	}
	for _, settingsPath := range settingsPaths {
		if fs.Has(settingsPath) {
			content := fs.Read(settingsPath)
			if len(content) > 0 && (
				filepath.Base(settingsPath) == "settings.py" || filepath.Base(settingsPath) == "base.py") {
				if strings.Contains(content, "ASGI_APPLICATION") ||
					strings.Contains(content, "asgi") {
					return "asgi"
				}
			}
		}
	}

	depFiles := []string{"requirements.txt", "pyproject.toml", "Pipfile"}
	for _, depFile := range depFiles {
		if fs.Has(depFile) {
			content := fs.Read(depFile)
			if strings.Contains(content, "uvicorn") ||
				strings.Contains(content, "daphne") ||
				strings.Contains(content, "channels") {
				return "asgi"
			}
		}
	}

	if fs.Has("wsgi.py") {
		return "wsgi"
	}

	for _, path := range []string{"config/wsgi.py", "core/wsgi.py", "mysite/wsgi.py", "project/wsgi.py"} {
		if fs.Has(path) {
			return "wsgi"
		}
	}

	return "wsgi"
}

func GetDjangoRunCommand(serverType, projectName string) string {
	if serverType == "asgi" {
		if projectName != "" {
			return "uvicorn " + projectName + ".asgi:application --host 0.0.0.0 --port 8000"
		}
		return "uvicorn asgi:application --host 0.0.0.0 --port 8000"
	}

	if projectName != "" {
		return "gunicorn " + projectName + ".wsgi:application --bind 0.0.0.0:8000 --workers 2"
	}
	return "gunicorn <yourproject>.wsgi:application --bind 0.0.0.0:8000 --workers 2"
}
