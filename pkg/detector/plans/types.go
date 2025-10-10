package plans

// FSReader provides filesystem operations for plan generation
type FSReader interface {
	Has(path string) bool
	Read(path string) string
	DirExists(path string) bool
	ContainsExt(files []string, ext string) bool
	ScanTree() ([]string, map[string]int, error)
}
