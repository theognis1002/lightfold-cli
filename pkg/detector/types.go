package detector

import "lightfold/pkg/detector/detectors"

// Detection represents the result of framework detection
type Detection struct {
	Framework   string            `json:"framework"`
	Language    string            `json:"language"`
	Confidence  float64           `json:"confidence"`
	Signals     []string          `json:"signals"`
	BuildPlan   []string          `json:"build_plan"`
	RunPlan     []string          `json:"run_plan"`
	Healthcheck map[string]any    `json:"healthcheck"`
	EnvSchema   []string          `json:"env_schema"`
	Meta        map[string]string `json:"meta,omitempty"`
}

// Candidate is an alias for detectors.Candidate
type Candidate = detectors.Candidate
