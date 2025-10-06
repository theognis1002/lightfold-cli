package detectors

// FSReader provides filesystem operations for detection
// This is a forward declaration that will be satisfied by detector.FSReader
type FSReader interface {
	Has(path string) bool
	Read(path string) string
	DirExists(path string) bool
	ContainsExt(files []string, ext string) bool
	ScanTree() ([]string, map[string]int, error)
}

// Candidate represents a framework detection candidate
type Candidate struct {
	Name     string
	Score    float64
	Language string
	Signals  []string
	Plan     any // Plan function that takes an FSReader and returns build, run, health, env, meta
}
