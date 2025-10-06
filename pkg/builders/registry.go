package builders

import (
	"fmt"
	"lightfold/pkg/detector"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	registry = make(map[string]BuilderFactory)
	mu       sync.RWMutex
)

// BuilderFactory is a function that creates a new Builder instance
type BuilderFactory func() Builder

// Register adds a builder to the registry
// Builders should call this in their init() function
func Register(name string, factory BuilderFactory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// GetBuilder retrieves a builder by name from the registry
func GetBuilder(name string) (Builder, error) {
	mu.RLock()
	defer mu.RUnlock()

	factory, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("unknown builder: %s", name)
	}

	return factory(), nil
}

// ListAvailableBuilders returns the names of all registered builders that are available
func ListAvailableBuilders() []string {
	mu.RLock()
	defer mu.RUnlock()

	available := []string{}
	for name, factory := range registry {
		builder := factory()
		if builder.IsAvailable() {
			available = append(available, name)
		}
	}
	return available
}

// AutoSelectBuilder determines the best builder for a project
// Priority order:
// 1. Dockerfile exists → use "dockerfile"
// 2. Node/Python + nixpacks available → use "nixpacks"
// 3. Fallback → use "native"
func AutoSelectBuilder(projectPath string, detection *detector.Detection) (string, error) {
	// Check for Dockerfile
	dockerfilePath := filepath.Join(projectPath, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); err == nil {
		if builder, err := GetBuilder("dockerfile"); err == nil && builder.IsAvailable() {
			return "dockerfile", nil
		}
	}

	// Check for Node/Python with nixpacks (only if detection is not nil)
	if detection != nil {
		lang := strings.ToLower(detection.Language)
		if lang == "javascript" || lang == "typescript" || lang == "python" {
			if builder, err := GetBuilder("nixpacks"); err == nil && builder.IsAvailable() {
				return "nixpacks", nil
			}
		}
	}

	// Fallback to native
	return "native", nil
}
