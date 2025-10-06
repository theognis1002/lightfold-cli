package detector_test

import (
	"testing"
)

// Runtime Version Detection Tests

func TestNodeVersionDetection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: ".nvmrc file",
			files: map[string]string{
				".nvmrc":       "18.17.0",
				"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":    "const express = require('express');",
			},
			expectedVersion: "18.17.0",
		},
		{
			name: ".node-version file",
			files: map[string]string{
				".node-version": "20.10.0",
				"package.json":  `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":     "const express = require('express');",
			},
			expectedVersion: "20.10.0",
		},
		{
			name: ".nvmrc takes priority over .node-version",
			files: map[string]string{
				".nvmrc":        "18.17.0",
				".node-version": "20.10.0",
				"package.json":  `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":     "const express = require('express');",
			},
			expectedVersion: "18.17.0",
		},
		{
			name: ".nvmrc with newline",
			files: map[string]string{
				".nvmrc":       "18.17.0\n",
				"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":    "const express = require('express');",
			},
			expectedVersion: "18.17.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Meta["runtime_version"] != tt.expectedVersion {
				t.Errorf("Expected runtime_version %s, got %s", tt.expectedVersion, detection.Meta["runtime_version"])
			}
		})
	}
}

func TestPythonVersionDetection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: ".python-version file",
			files: map[string]string{
				".python-version":  "3.11.5",
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
			expectedVersion: "3.11.5",
		},
		{
			name: "runtime.txt file (Heroku style)",
			files: map[string]string{
				"runtime.txt":      "python-3.11.0",
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
			expectedVersion: "3.11.0",
		},
		{
			name: ".python-version takes priority over runtime.txt",
			files: map[string]string{
				".python-version":  "3.11.5",
				"runtime.txt":      "python-3.10.0",
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
			expectedVersion: "3.11.5",
		},
		{
			name: ".python-version with newline",
			files: map[string]string{
				".python-version":  "3.11.5\n",
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
			expectedVersion: "3.11.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Meta["runtime_version"] != tt.expectedVersion {
				t.Errorf("Expected runtime_version %s, got %s", tt.expectedVersion, detection.Meta["runtime_version"])
			}
		})
	}
}

func TestRubyVersionDetection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: ".ruby-version file",
			files: map[string]string{
				".ruby-version":         "3.2.0",
				"bin/rails":             "#!/usr/bin/env ruby\nrequire 'rails'",
				"Gemfile.lock":          "GEM\n  remote: https://rubygems.org/",
				"config/application.rb": "require 'rails/all'",
			},
			expectedVersion: "3.2.0",
		},
		{
			name: ".ruby-version with newline",
			files: map[string]string{
				".ruby-version":         "3.2.0\n",
				"bin/rails":             "#!/usr/bin/env ruby\nrequire 'rails'",
				"Gemfile.lock":          "GEM\n  remote: https://rubygems.org/",
				"config/application.rb": "require 'rails/all'",
			},
			expectedVersion: "3.2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Meta["runtime_version"] != tt.expectedVersion {
				t.Errorf("Expected runtime_version %s, got %s", tt.expectedVersion, detection.Meta["runtime_version"])
			}
		})
	}
}

func TestGoVersionDetection(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedVersion string
	}{
		{
			name: ".go-version file",
			files: map[string]string{
				".go-version": "1.21.5",
				"go.mod":      "module example.com/myapp\ngo 1.21",
				"main.go":     "package main\nfunc main() {}",
			},
			expectedVersion: "1.21.5",
		},
		{
			name: ".go-version with newline",
			files: map[string]string{
				".go-version": "1.21.5\n",
				"go.mod":      "module example.com/myapp\ngo 1.21",
				"main.go":     "package main\nfunc main() {}",
			},
			expectedVersion: "1.21.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Meta["runtime_version"] != tt.expectedVersion {
				t.Errorf("Expected runtime_version %s, got %s", tt.expectedVersion, detection.Meta["runtime_version"])
			}
		})
	}
}

func TestNoRuntimeVersionDetection(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name: "Node project without version file",
			files: map[string]string{
				"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":    "const express = require('express');",
			},
		},
		{
			name: "Python project without version file",
			files: map[string]string{
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
		},
		{
			name: "Go project without version file",
			files: map[string]string{
				"go.mod":  "module example.com/myapp\ngo 1.21",
				"main.go": "package main\nfunc main() {}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if _, exists := detection.Meta["runtime_version"]; exists {
				t.Errorf("Expected no runtime_version, but got %s", detection.Meta["runtime_version"])
			}
		})
	}
}

func TestEmptyRuntimeVersionFile(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name: "Empty .nvmrc file",
			files: map[string]string{
				".nvmrc":       "",
				"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
				"server.js":    "const express = require('express');",
			},
		},
		{
			name: "Whitespace-only .python-version file",
			files: map[string]string{
				".python-version":  "  \n  ",
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			// Empty or whitespace-only versions should not be added to meta
			if version, exists := detection.Meta["runtime_version"]; exists && version != "" {
				t.Errorf("Expected no runtime_version for empty file, but got %s", version)
			}
		})
	}
}
