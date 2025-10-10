package plans

import "lightfold/pkg/config"

// DefaultHealthcheck returns a standard healthcheck configuration
func DefaultHealthcheck(path string) map[string]any {
	return map[string]any{
		"path":            path,
		"expect":          config.DefaultHealthCheckStatus,
		"timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds()),
	}
}

// MergeDeps merges multiple dependency maps into a single map
func MergeDeps(deps ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, d := range deps {
		for k, v := range d {
			merged[k] = v
		}
	}
	return merged
}
