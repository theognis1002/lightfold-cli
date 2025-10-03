package util

import (
	"fmt"
	"os"
)

// LoadEnvFile reads and parses a .env file into a map of environment variables
func LoadEnvFile(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	envVars := make(map[string]string)
	lines := SplitLines(string(data))

	for i, line := range lines {
		line = TrimSpace(line)
		if line == "" || StartsWithHash(line) {
			continue
		}

		parts := SplitEnvVar(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env var at line %d: %s", i+1, line)
		}

		value := parts[1]
		// Remove surrounding quotes if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		envVars[parts[0]] = value
	}

	return envVars, nil
}

// SplitEnvVar splits an environment variable string on the first '=' character
func SplitEnvVar(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '=' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

// SplitLines splits a string into lines by '\n' character
func SplitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TrimSpace removes leading and trailing whitespace characters
func TrimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// StartsWithHash checks if a string starts with '#' (comment line)
func StartsWithHash(s string) bool {
	return len(s) > 0 && s[0] == '#'
}
