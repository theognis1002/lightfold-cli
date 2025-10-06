package detectors

import (
	"lightfold/pkg/detector/plans"
)

// DetectPython detects Python frameworks
func DetectPython(fs FSReader) []Candidate {
	var candidates []Candidate

	if c := detectDjango(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFlask(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	if c := detectFastAPI(fs); c.Score > 0 {
		candidates = append(candidates, c)
	}

	return candidates
}

func detectDjango(fs FSReader) Candidate {
	return NewDetectionBuilder("Django", "Python", fs).
		CheckFile("manage.py", ScoreConfigFile, "manage.py").
		CheckAnyFile([]string{"requirements.txt", "Pipfile.lock", "poetry.lock", "pyproject.toml"}, ScoreLockfile, "python deps lockfile").
		CheckAnyFile([]string{"myproject/wsgi.py", "wsgi.py", "asgi.py"}, ScoreStructure, "wsgi/asgi").
		CheckMultipleContent([]string{"requirements.txt", "pyproject.toml"}, "django", ScoreLockfile, "mentions django in deps").
		Build(plans.DjangoPlan)
}

func detectFlask(fs FSReader) Candidate {
	return NewDetectionBuilder("Flask", "Python", fs).
		CheckAnyFile([]string{"app.py", "wsgi.py", "application.py"}, ScoreLockfile, "Flask app file").
		CheckMultipleContent([]string{"requirements.txt", "Pipfile", "pyproject.toml"}, "flask", ScoreDependency, "Flask in dependencies").
		CheckDir("templates", ScoreMinorIndicator, "templates/ folder").
		Build(plans.FlaskPlan)
}

func detectFastAPI(fs FSReader) Candidate {
	return NewDetectionBuilder("FastAPI", "Python", fs).
		CheckMultipleContent([]string{"main.py", "app.py"}, "fastapi", ScoreConfigFile, "FastAPI import in main/app file").
		CheckMultipleContent([]string{"requirements.txt", "pyproject.toml"}, "fastapi", ScoreDependency, "FastAPI in dependencies").
		Build(plans.FastAPIPlan)
}
