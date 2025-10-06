package detector

import (
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// FSReader provides filesystem operations abstracted over fs.FS
type FSReader struct {
	fsys fs.FS
}

// NewFSReader creates a new FSReader for the given filesystem
func NewFSReader(fsys fs.FS) *FSReader {
	return &FSReader{fsys: fsys}
}

// Has checks if a file exists at the given path
func (r *FSReader) Has(path string) bool {
	_, err := fs.Stat(r.fsys, path)
	return err == nil
}

// Read reads a file and returns its content as a string
func (r *FSReader) Read(path string) string {
	f, err := r.fsys.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(data)
}

// DirExists checks if a directory exists at the given path
func (r *FSReader) DirExists(path string) bool {
	fi, err := fs.Stat(r.fsys, path)
	return err == nil && fi.IsDir()
}

// ContainsExt checks if the file list contains files with the given extension
func (r *FSReader) ContainsExt(files []string, ext string) bool {
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ext) {
			return true
		}
	}
	return false
}

// ScanTree walks the filesystem and returns all files and extension counts
func (r *FSReader) ScanTree() ([]string, map[string]int, error) {
	var files []string
	extCounts := map[string]int{}

	err := fs.WalkDir(r.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		base := filepath.Base(p)
		if d.IsDir() && (base == ".git" || base == "node_modules" || base == ".venv" || base == "venv" || base == "dist" || base == "build") {
			return fs.SkipDir
		}

		if !d.IsDir() {
			files = append(files, p)
			ext := strings.ToLower(filepath.Ext(p))
			if ext != "" {
				extCounts[ext]++
			}
		}
		return nil
	})

	return files, extCounts, err
}
