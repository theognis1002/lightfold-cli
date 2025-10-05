package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lightfold/pkg/detector/detectors"
)

func DetectFramework(root string) Detection {
	allFiles, extCounts, err := scanTree(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		os.Exit(1)
	}

	helpers := detectors.HelperFuncs{
		Has: func(rel string) bool {
			return fileExists(root, rel)
		},
		Read: func(rel string) string {
			b, _ := os.ReadFile(filepath.Join(root, rel))
			return string(b)
		},
		DirExists: func(root, rel string) bool {
			return dirExists(root, rel)
		},
		ContainsExt: func(files []string, ext string) bool {
			return containsExt(files, ext)
		},
	}

	var cands []detectors.Candidate

	cands = append(cands, detectors.DetectPython(root, helpers)...)
	cands = append(cands, detectors.DetectJavaScript(root, allFiles, helpers)...)
	cands = append(cands, detectors.DetectGo(root, allFiles, helpers)...)
	cands = append(cands, detectors.DetectPHP(root, helpers)...)
	cands = append(cands, detectors.DetectRuby(root, helpers)...)
	cands = append(cands, detectors.DetectJava(root, helpers)...)
	cands = append(cands, detectors.DetectElixir(root, helpers)...)
	cands = append(cands, detectors.DetectRust(root, allFiles, helpers)...)
	cands = append(cands, detectors.DetectCSharp(root, allFiles, helpers)...)
	cands = append(cands, detectors.DetectDocker(root, dominantLanguage(extCounts), helpers)...)

	if len(cands) == 0 {
		lang := dominantLanguage(extCounts)
		meta := map[string]string{"note": "Fell back to generic. Provide custom commands."}

		if runtimeVersion := detectRuntimeVersion(root, lang); runtimeVersion != "" {
			meta["runtime_version"] = runtimeVersion
		}

		monorepoMeta := detectMonorepo(root)
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

	build, run, health, env, meta := best.Plan(root)

	if runtimeVersion := detectRuntimeVersion(root, best.Language); runtimeVersion != "" {
		meta["runtime_version"] = runtimeVersion
	}

	monorepoMeta := detectMonorepo(root)
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

func DetectAndPrint(root string) {
	detection := DetectFramework(root)
	emitJSON(detection)
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
