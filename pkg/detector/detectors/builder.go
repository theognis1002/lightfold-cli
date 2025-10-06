package detectors

import "strings"

// DetectionBuilder provides a fluent API for building framework detection candidates
// It eliminates boilerplate by automatically managing score accumulation and signal collection
type DetectionBuilder struct {
	name     string
	language string
	score    float64
	signals  []string
	fs       FSReader
}

// NewDetectionBuilder creates a new detection builder for a framework
func NewDetectionBuilder(name, language string, fs FSReader) *DetectionBuilder {
	return &DetectionBuilder{
		name:     name,
		language: language,
		score:    0.0,
		signals:  []string{},
		fs:       fs,
	}
}

// CheckFile checks if a file exists and adds score/signal if found
func (b *DetectionBuilder) CheckFile(path string, score float64, signal string) *DetectionBuilder {
	if b.fs.Has(path) {
		b.score += score
		b.signals = append(b.signals, signal)
	}
	return b
}

// CheckAnyFile checks if any of the provided files exist and adds score/signal if found
func (b *DetectionBuilder) CheckAnyFile(paths []string, score float64, signal string) *DetectionBuilder {
	for _, path := range paths {
		if b.fs.Has(path) {
			b.score += score
			b.signals = append(b.signals, signal)
			return b
		}
	}
	return b
}

// CheckDependency checks if a dependency exists in a file (case-insensitive)
func (b *DetectionBuilder) CheckDependency(filePath, dependency string, score float64, signal string) *DetectionBuilder {
	if b.fs.Has(filePath) {
		content := strings.ToLower(b.fs.Read(filePath))
		if strings.Contains(content, strings.ToLower(dependency)) {
			b.score += score
			b.signals = append(b.signals, signal)
		}
	}
	return b
}

// CheckContent checks if content contains a substring (case-insensitive)
func (b *DetectionBuilder) CheckContent(filePath, substring string, score float64, signal string) *DetectionBuilder {
	if b.fs.Has(filePath) {
		content := strings.ToLower(b.fs.Read(filePath))
		if strings.Contains(content, strings.ToLower(substring)) {
			b.score += score
			b.signals = append(b.signals, signal)
		}
	}
	return b
}

// CheckMultipleContent checks multiple files for a substring (case-insensitive)
// Adds score/signal only once if found in any file
func (b *DetectionBuilder) CheckMultipleContent(filePaths []string, substring string, score float64, signal string) *DetectionBuilder {
	for _, filePath := range filePaths {
		if b.fs.Has(filePath) {
			content := strings.ToLower(b.fs.Read(filePath))
			if strings.Contains(content, strings.ToLower(substring)) {
				b.score += score
				b.signals = append(b.signals, signal)
				return b
			}
		}
	}
	return b
}

// CheckDir checks if a directory exists and adds score/signal if found
func (b *DetectionBuilder) CheckDir(path string, score float64, signal string) *DetectionBuilder {
	if b.fs.DirExists(path) {
		b.score += score
		b.signals = append(b.signals, signal)
	}
	return b
}

// CheckAnyDir checks if any of the provided directories exist and adds score/signal if found
func (b *DetectionBuilder) CheckAnyDir(paths []string, score float64, signal string) *DetectionBuilder {
	for _, path := range paths {
		if b.fs.DirExists(path) {
			b.score += score
			b.signals = append(b.signals, signal)
			return b
		}
	}
	return b
}

// CheckExtension checks if files with a specific extension exist
func (b *DetectionBuilder) CheckExtension(allFiles []string, ext string, score float64, signal string) *DetectionBuilder {
	if b.fs.ContainsExt(allFiles, ext) {
		b.score += score
		b.signals = append(b.signals, signal)
	}
	return b
}

// CheckCondition adds score/signal if a custom condition is met
func (b *DetectionBuilder) CheckCondition(condition bool, score float64, signal string) *DetectionBuilder {
	if condition {
		b.score += score
		b.signals = append(b.signals, signal)
	}
	return b
}

// Build finalizes the builder and returns a Candidate
func (b *DetectionBuilder) Build(plan any) Candidate {
	return Candidate{
		Name:     b.name,
		Score:    b.score,
		Language: b.language,
		Signals:  b.signals,
		Plan:     plan,
	}
}

// GetScore returns the current score (useful for conditional logic)
func (b *DetectionBuilder) GetScore() float64 {
	return b.score
}
