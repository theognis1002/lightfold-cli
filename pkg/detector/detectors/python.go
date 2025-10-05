package detectors

import (
	"strings"

	"lightfold/pkg/detector/plans"
)

// DetectPython detects Python frameworks
func DetectPython(root string, helpers HelperFuncs) []Candidate {
	var candidates []Candidate

	if c := detectDjango(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFlask(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFastAPI(root, helpers); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectDjango(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("manage.py") {
		score += 3
		signals = append(signals, "manage.py")
	}
	if h.Has("requirements.txt") || h.Has("Pipfile.lock") || h.Has("poetry.lock") || h.Has("pyproject.toml") {
		score += 1.5
		signals = append(signals, "python deps lockfile")
	}
	if h.Has("myproject/wsgi.py") || h.Has("wsgi.py") || h.Has("asgi.py") {
		score += 1
		signals = append(signals, "wsgi/asgi")
	}
	if strings.Contains(strings.ToLower(h.Read("requirements.txt")), "django") || strings.Contains(strings.ToLower(h.Read("pyproject.toml")), "django") {
		score += 1.5
		signals = append(signals, "mentions django in deps")
	}

	return Candidate{
		Name:     "Django",
		Score:    score,
		Language: "Python",
		Signals:  signals,
		Plan:     plans.DjangoPlan,
	}
}

func detectFlask(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("app.py") || h.Has("wsgi.py") || h.Has("application.py") {
		score += 2
		signals = append(signals, "Flask app file")
	}
	if h.Has("requirements.txt") || h.Has("Pipfile") || h.Has("Pipfile") || h.Has("pyproject.toml") {
		content := strings.ToLower(h.Read("requirements.txt") + h.Read("Pipfile") + h.Read("pyproject.toml"))
		if strings.Contains(content, "flask") {
			score += 2.5
			signals = append(signals, "Flask in dependencies")
		}
	}
	if h.Has("templates") && h.DirExists(root, "templates") {
		score += 0.5
		signals = append(signals, "templates/ folder")
	}

	return Candidate{
		Name:     "Flask",
		Score:    score,
		Language: "Python",
		Signals:  signals,
		Plan:     plans.FlaskPlan,
	}
}

func detectFastAPI(root string, h HelperFuncs) Candidate {
	score := 0.0
	signals := []string{}

	if h.Has("main.py") || h.Has("app.py") {
		content := strings.ToLower(h.Read("main.py") + h.Read("app.py"))
		if strings.Contains(content, "fastapi") {
			score += 3
			signals = append(signals, "FastAPI import in main/app file")
		}
	}
	if h.Has("requirements.txt") || h.Has("pyproject.toml") {
		content := strings.ToLower(h.Read("requirements.txt") + h.Read("pyproject.toml"))
		if strings.Contains(content, "fastapi") {
			score += 2.5
			signals = append(signals, "FastAPI in dependencies")
		}
	}

	return Candidate{
		Name:     "FastAPI",
		Score:    score,
		Language: "Python",
		Signals:  signals,
		Plan:     plans.FastAPIPlan,
	}
}
