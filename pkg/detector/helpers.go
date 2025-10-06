package detector

import (
	"strings"
)

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
func pickBest(cands []Candidate) Candidate {
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
func detectRuntimeVersion(fs *FSReader, language string) string {
	switch language {
	case "JavaScript/TypeScript":
		if fs.Has(".nvmrc") {
			return strings.TrimSpace(fs.Read(".nvmrc"))
		}
		if fs.Has(".node-version") {
			return strings.TrimSpace(fs.Read(".node-version"))
		}
	case "Python":
		if fs.Has(".python-version") {
			return strings.TrimSpace(fs.Read(".python-version"))
		}
		if fs.Has("runtime.txt") {
			content := strings.TrimSpace(fs.Read("runtime.txt"))
			// Parse "python-3.11.0" format
			return strings.TrimPrefix(content, "python-")
		}
	case "Ruby":
		if fs.Has(".ruby-version") {
			return strings.TrimSpace(fs.Read(".ruby-version"))
		}
	case "Go":
		if fs.Has(".go-version") {
			return strings.TrimSpace(fs.Read(".go-version"))
		}
	}
	return ""
}

// detectMonorepo detects monorepo tools and configuration
func detectMonorepo(fs *FSReader) map[string]string {
	result := map[string]string{}

	if fs.Has("turbo.json") {
		result["monorepo_tool"] = "turborepo"
	} else if fs.Has("nx.json") {
		result["monorepo_tool"] = "nx"
	} else if fs.Has("lerna.json") {
		result["monorepo_tool"] = "lerna"
	} else if fs.Has("pnpm-workspace.yaml") {
		result["monorepo_tool"] = "pnpm-workspaces"
	} else if fs.Has("package.json") {
		content := fs.Read("package.json")
		if strings.Contains(content, `"workspaces"`) {
			result["monorepo_tool"] = "yarn-workspaces"
		}
	}

	if result["monorepo_tool"] != "" {
		result["monorepo"] = "true"
	}

	return result
}
