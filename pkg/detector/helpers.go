package detector

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"lightfold/pkg/detector/detectors"
)

// scanTree walks the directory tree and returns all files and extension counts
func scanTree(root string) ([]string, map[string]int, error) {
	var files []string
	extCounts := map[string]int{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		base := filepath.Base(p)
		if d.IsDir() && (base == ".git" || base == "node_modules" || base == ".venv" || base == "venv" || base == "dist" || base == "build") {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			files = append(files, rel)
			ext := strings.ToLower(filepath.Ext(p))
			if ext != "" {
				extCounts[ext]++
			}
		}
		return nil
	})
	return files, extCounts, err
}

// containsExt checks if the file list contains files with the given extension
func containsExt(files []string, ext string) bool {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ext) {
			return true
		}
	}
	return false
}

// fileExists checks if a file exists at the given path
func fileExists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

// dirExists checks if a directory exists at the given path
func dirExists(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, rel))
	return err == nil && fi.IsDir()
}

// readFile reads a file and returns its content as a string
func readFile(root, rel string) string {
	b, _ := os.ReadFile(filepath.Join(root, rel))
	return string(b)
}

// dominantLanguage determines the dominant language from extension counts
func dominantLanguage(extCounts map[string]int) string {
	langScores := map[string]int{}
	for ext, n := range extCounts {
		lang := mapExt(ext)
		langScores[lang] += n
	}
	bestLang := "Unknown"
	best := 0
	for lang, n := range langScores {
		if n > best {
			best = n
			bestLang = lang
		}
	}
	return bestLang
}

// mapExt maps file extensions to language names
func mapExt(ext string) string {
	switch ext {
	case ".py":
		return "Python"
	case ".js", ".jsx", ".ts", ".tsx":
		return "JavaScript/TypeScript"
	case ".php":
		return "PHP"
	case ".rb":
		return "Ruby"
	case ".go":
		return "Go"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".cs":
		return "C#"
	case ".ex", ".exs":
		return "Elixir"
	case ".vue":
		return "JavaScript/TypeScript"
	case ".svelte":
		return "JavaScript/TypeScript"
	default:
		return "Other"
	}
}

// clamp constrains a value between lo and hi
func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// pickBest selects the best candidate from the list
func pickBest(cands []detectors.Candidate) detectors.Candidate {
	best := cands[0]
	for _, c := range cands[1:] {
		if c.Score > best.Score {
			best = c
			continue
		}
		if c.Score == best.Score {
			if best.Name == "Generic Docker" && c.Name != "Generic Docker" {
				best = c
			}
		}
	}
	return best
}

// detectRuntimeVersion detects runtime version from version files
func detectRuntimeVersion(root string, language string) string {
	switch language {
	case "JavaScript/TypeScript":
		if fileExists(root, ".nvmrc") {
			return strings.TrimSpace(readFile(root, ".nvmrc"))
		}
		if fileExists(root, ".node-version") {
			return strings.TrimSpace(readFile(root, ".node-version"))
		}
	case "Python":
		if fileExists(root, ".python-version") {
			return strings.TrimSpace(readFile(root, ".python-version"))
		}
		if fileExists(root, "runtime.txt") {
			content := strings.TrimSpace(readFile(root, "runtime.txt"))
			// Parse "python-3.11.0" format
			return strings.TrimPrefix(content, "python-")
		}
	case "Ruby":
		if fileExists(root, ".ruby-version") {
			return strings.TrimSpace(readFile(root, ".ruby-version"))
		}
	case "Go":
		if fileExists(root, ".go-version") {
			return strings.TrimSpace(readFile(root, ".go-version"))
		}
	}
	return ""
}

// detectMonorepo detects monorepo tools and configuration
func detectMonorepo(root string) map[string]string {
	result := map[string]string{}

	if fileExists(root, "turbo.json") {
		result["monorepo_tool"] = "turborepo"
	} else if fileExists(root, "nx.json") {
		result["monorepo_tool"] = "nx"
	} else if fileExists(root, "lerna.json") {
		result["monorepo_tool"] = "lerna"
	} else if fileExists(root, "pnpm-workspace.yaml") {
		result["monorepo_tool"] = "pnpm-workspaces"
	} else if fileExists(root, "package.json") {
		content := readFile(root, "package.json")
		if strings.Contains(content, `"workspaces"`) {
			result["monorepo_tool"] = "yarn-workspaces"
		}
	}

	if result["monorepo_tool"] != "" {
		result["monorepo"] = "true"
	}

	return result
}
