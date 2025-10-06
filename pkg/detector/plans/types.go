package plans

// FSReader provides filesystem operations for plan generation
type FSReader interface {
	Has(path string) bool
	Read(path string) string
	DirExists(path string) bool
}
