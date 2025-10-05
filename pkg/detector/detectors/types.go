package detectors

// Candidate represents a framework detection candidate
type Candidate struct {
	Name     string
	Score    float64
	Language string
	Signals  []string
	Plan     func(root string) (build []string, run []string, health map[string]any, env []string, meta map[string]string)
}

// HelperFuncs contains helper functions for detection
type HelperFuncs struct {
	Has         func(string) bool
	Read        func(string) string
	DirExists   func(string, string) bool
	ContainsExt func([]string, string) bool
}
