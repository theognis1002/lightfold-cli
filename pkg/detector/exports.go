package detector

import (
	"lightfold/pkg/detector/packagemanagers"
	"lightfold/pkg/detector/plans"
	"os"
)

func DetectPackageManager(root string) string {
	reader := NewFSReader(os.DirFS(root))
	return packagemanagers.DetectJS(reader)
}

func DetectPythonPackageManager(root string) string {
	reader := NewFSReader(os.DirFS(root))
	return packagemanagers.DetectPython(reader)
}

func GetJSInstallCommand(pm string) string {
	return packagemanagers.GetJSInstallCommand(pm)
}

func GetJSBuildCommand(pm string) string {
	return packagemanagers.GetJSBuildCommand(pm)
}

func GetJSStartCommand(pm string) string {
	return packagemanagers.GetJSStartCommand(pm)
}

func GetPythonInstallCommand(pm string) string {
	return packagemanagers.GetPythonInstallCommand(pm)
}

func NextPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	reader := NewFSReader(os.DirFS(root))
	return plans.NextPlan(reader)
}

func DjangoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	reader := NewFSReader(os.DirFS(root))
	return plans.DjangoPlan(reader)
}

func GoPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	reader := NewFSReader(os.DirFS(root))
	return plans.GoPlan(reader)
}