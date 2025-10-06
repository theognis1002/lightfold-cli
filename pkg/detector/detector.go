package detector

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"lightfold/pkg/detector/detectors"
	"lightfold/pkg/detector/plans"
)

// DetectFrameworkFS detects the framework from a filesystem abstraction
func DetectFrameworkFS(fsys fs.FS) Detection {
	reader := NewFSReader(fsys)

	allFiles, extCounts, err := reader.ScanTree()
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		os.Exit(1)
	}

	var cands []detectors.Candidate

	cands = append(cands, detectors.DetectPython(reader)...)
	cands = append(cands, detectors.DetectJavaScript(reader, allFiles)...)
	cands = append(cands, detectors.DetectGo(reader, allFiles)...)
	cands = append(cands, detectors.DetectPHP(reader)...)
	cands = append(cands, detectors.DetectRuby(reader)...)
	cands = append(cands, detectors.DetectJava(reader)...)
	cands = append(cands, detectors.DetectElixir(reader)...)
	cands = append(cands, detectors.DetectRust(reader, allFiles)...)
	cands = append(cands, detectors.DetectCSharp(reader, allFiles)...)
	cands = append(cands, detectors.DetectDocker(reader, dominantLanguage(extCounts))...)

	if len(cands) == 0 {
		lang := dominantLanguage(extCounts)
		meta := map[string]string{"note": "Fell back to generic. Provide custom commands."}

		if runtimeVersion := detectRuntimeVersion(reader, lang); runtimeVersion != "" {
			meta["runtime_version"] = runtimeVersion
		}

		monorepoMeta := detectMonorepo(reader)
		for k, v := range monorepoMeta {
			meta[k] = v
		}

		out := Detection{
			Framework:   "Unknown",
			Language:    lang,
			Confidence:  0.0,
			Signals:     []string{"no strong framework signals"},
			BuildPlan:   []string{"# Please provide build steps"},
			RunPlan:     []string{"# Please provide run command"},
			Healthcheck: map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30},
			EnvSchema:   []string{},
			Meta:        meta,
		}
		return out
	}

	best := pickBest(cands)

	// Call the plan function - it takes the plans.FSReader interface
	var build, run []string
	var health map[string]any
	var env []string
	var meta map[string]string

	if planFunc, ok := best.Plan.(func(plans.FSReader) ([]string, []string, map[string]any, []string, map[string]string)); ok {
		build, run, health, env, meta = planFunc(reader)
	} else {
		return Detection{
			Framework:   best.Name,
			Language:    best.Language,
			Confidence:  0,
			Signals:     best.Signals,
			BuildPlan:   []string{},
			RunPlan:     []string{},
			Healthcheck: map[string]any{"path": "/", "expect": 200, "timeout_seconds": 30},
			EnvSchema:   []string{},
			Meta:        map[string]string{},
		}
	}

	if runtimeVersion := detectRuntimeVersion(reader, best.Language); runtimeVersion != "" {
		meta["runtime_version"] = runtimeVersion
	}

	monorepoMeta := detectMonorepo(reader)
	for k, v := range monorepoMeta {
		meta[k] = v
	}

	out := Detection{
		Framework:   best.Name,
		Language:    best.Language,
		Confidence:  clamp(best.Score/6.0, 0, 1),
		Signals:     best.Signals,
		BuildPlan:   build,
		RunPlan:     run,
		Healthcheck: health,
		EnvSchema:   env,
		Meta:        meta,
	}
	return out
}

// DetectFramework detects the framework from a local filesystem path (CLI use)
func DetectFramework(root string) Detection {
	return DetectFrameworkFS(os.DirFS(root))
}

func DetectAndPrint(root string) {
	detection := DetectFramework(root)
	emitJSON(detection)
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
