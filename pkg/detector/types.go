package detector

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

// candidate represents a framework candidate during detection
type candidate struct {
	name     string
	score    float64
	language string
	signals  []string
	plan     func(root string) (build []string, run []string, health map[string]any, env []string, meta map[string]string)
}
