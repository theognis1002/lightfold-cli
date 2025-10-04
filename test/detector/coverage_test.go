package detector_test

import (
	"testing"
)

// Test edge cases and additional coverage scenarios

func TestEdgeCasesAndCoverage(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
	}{
		{
			name: "Django with Pipfile instead of requirements.txt",
			files: map[string]string{
				"manage.py": "#!/usr/bin/env python\nimport django",
				"Pipfile": `[[source]]
url = "https://pypi.org/simple"
verify_ssl = true
name = "pypi"

[packages]
django = "*"
psycopg2-binary = "*"

[dev-packages]
pytest = "*"

[requires]
python_version = "3.9"`,
				"Pipfile.lock": `{"_meta": {"requires": {"python_version": "3.9"}}}`,
			},
			expectedFramework: "Django",
			expectedLanguage:  "Python",
		},
		{
			name: "Express.js with index.js as entry point",
			files: map[string]string{
				"package.json": `{
  "main": "index.js",
  "dependencies": {"express": "^4.18.0"},
  "scripts": {"start": "node index.js"}
}`,
				"index.js": `const express = require('express');
const app = express();
app.get('/', (req, res) => res.send('Hello World!'));
app.listen(3000);`,
			},
			expectedFramework: "Express.js",
			expectedLanguage:  "JavaScript/TypeScript",
		},
		{
			name: "Flask with application.py entry point",
			files: map[string]string{
				"application.py": `from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(debug=True)`,
				"requirements.txt": "Flask==2.3.2\nWerkzeug==2.3.6",
				"templates/index.html": "<html><body><h1>Flask App</h1></body></html>",
			},
			expectedFramework: "Flask",
			expectedLanguage:  "Python",
		},
		{
			name: "Spring Boot with Gradle instead of Maven",
			files: map[string]string{
				"build.gradle": `plugins {
    id 'org.springframework.boot' version '3.1.0'
    id 'io.spring.dependency-management' version '1.1.0'
    id 'java'
}

group = 'com.example'
version = '0.0.1-SNAPSHOT'
sourceCompatibility = '17'

repositories {
    mavenCentral()
}

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web'
    testImplementation 'org.springframework.boot:spring-boot-starter-test'
}`,
				"src/main/java/com/example/demo/DemoApplication.java": `package com.example.demo;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class DemoApplication {
    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }
}`,
			},
			expectedFramework: "Spring Boot",
			expectedLanguage:  "Java",
		},
		{
			name: "Phoenix with typical Elixir structure",
			files: map[string]string{
				"mix.exs": `defmodule MyAppWeb.MixProject do
  use Mix.Project

  def project do
    [
      app: :my_app_web,
      version: "0.1.0",
      elixir: "~> 1.14",
      deps: deps()
    ]
  end

  defp deps do
    [
      {:phoenix, "~> 1.7.0"},
      {:phoenix_html, "~> 3.3"},
      {:phoenix_live_reload, "~> 1.2", only: :dev},
      {:phoenix_live_view, "~> 0.18.1"},
      {:floki, ">= 0.30.0", only: :test},
      {:phoenix_live_dashboard, "~> 0.7.2"},
      {:telemetry_metrics, "~> 0.6"},
      {:telemetry_poller, "~> 1.0"},
      {:gettext, "~> 0.20"},
      {:jason, "~> 1.2"},
      {:plug_cowboy, "~> 2.5"}
    ]
  end
end`,
				"lib/my_app_web/application.ex": `defmodule MyAppWeb.Application do
  use Application

  def start(_type, _args) do
    children = [
      MyAppWebWeb.Endpoint
    ]

    opts = [strategy: :one_for_one, name: MyAppWeb.Supervisor]
    Supervisor.start_link(children, opts)
  end
end`,
				"lib/my_app_web_web/endpoint.ex": `defmodule MyAppWebWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :my_app_web
end`,
				"priv/repo/migrations/.gitkeep": "",
			},
			expectedFramework: "Phoenix",
			expectedLanguage:  "Elixir",
		},
		{
			name: "Generic Docker with unknown language",
			files: map[string]string{
				"Dockerfile": `FROM alpine:latest
RUN apk add --no-cache curl
COPY script.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/script.sh"]`,
				"script.sh": "#!/bin/sh\necho 'Hello from Docker'",
				"config.yaml": "version: 1\nname: myapp",
			},
			expectedFramework: "Generic Docker",
			expectedLanguage:  "Other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			if detection.Language != tt.expectedLanguage {
				t.Errorf("Expected language %s, got %s", tt.expectedLanguage, detection.Language)
			}

			// Ensure build plan is not empty for known frameworks
			if tt.expectedFramework != "Unknown" && len(detection.BuildPlan) == 0 {
				t.Error("Expected non-empty build plan for known framework")
			}

			// Ensure run plan is not empty for known frameworks
			if tt.expectedFramework != "Unknown" && len(detection.RunPlan) == 0 {
				t.Error("Expected non-empty run plan for known framework")
			}
		})
	}
}

func TestLanguageDetectionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected string
	}{
		{
			name: "Mixed language project - JavaScript dominant",
			files: map[string]string{
				"src/main.js":        "console.log('hello');",
				"src/component.jsx":  "export default function Component() {}",
				"src/utils.ts":       "export const helper = () => {};",
				"src/types.tsx":      "export interface Props {}",
				"server.py":          "import flask",
				"config.yaml":        "version: 1",
			},
			expected: "JavaScript/TypeScript",
		},
		{
			name: "Python dominant project",
			files: map[string]string{
				"src/main.py":        "print('hello')",
				"src/models.py":      "class User: pass",
				"src/views.py":       "def index(): return 'hello'",
				"src/utils.py":       "def helper(): pass",
				"src/helpers.py":     "def util(): pass",
				"src/services.py":    "def service(): pass",
				"src/handlers.py":    "def handle(): pass",
				"config.py":          "DEBUG = True",
				"requirements.txt":   "django==4.2.0",
				"package.json":       "{}",
				"index.js":           "console.log('test');",
			},
			expected: "Python",
		},
		{
			name: "Go project with multiple files",
			files: map[string]string{
				"main.go":            "package main",
				"handlers/user.go":   "package handlers",
				"models/user.go":     "package models",
				"utils/helper.go":    "package utils",
				"cmd/server/main.go": "package main",
			},
			expected: "Go",
		},
		{
			name: "Ruby project",
			files: map[string]string{
				"app.rb":             "puts 'hello'",
				"models/user.rb":     "class User; end",
				"controllers/app.rb": "class AppController; end",
			},
			expected: "Ruby",
		},
		{
			name: "PHP project",
			files: map[string]string{
				"index.php":          "<?php echo 'hello'; ?>",
				"models/User.php":    "<?php class User {}",
				"config/app.php":     "<?php return [];",
			},
			expected: "PHP",
		},
		{
			name: "C# project",
			files: map[string]string{
				"Program.cs":         "using System;",
				"Models/User.cs":     "public class User {}",
				"Controllers/App.cs": "public class AppController {}",
			},
			expected: "C#",
		},
		{
			name: "Java project",
			files: map[string]string{
				"src/Main.java":      "public class Main {}",
				"src/User.java":      "public class User {}",
				"src/Helper.java":    "public class Helper {}",
			},
			expected: "Java",
		},
		{
			name: "Elixir project",
			files: map[string]string{
				"lib/app.ex":         "defmodule App do end",
				"lib/user.ex":        "defmodule User do end",
				"test/app_test.exs":  "defmodule AppTest do end",
			},
			expected: "Elixir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Language != tt.expected {
				t.Errorf("Expected language %s, got %s", tt.expected, detection.Language)
			}
		})
	}
}

func TestPackageManagerBuildPlans(t *testing.T) {
	tests := []struct {
		name               string
		files              map[string]string
		expectedFramework  string
		expectedInstallCmd string
		expectedBuildCmd   string
	}{
		{
			name: "Next.js with yarn",
			files: map[string]string{
				"next.config.js": "module.exports = {}",
				"package.json":   `{"dependencies": {"next": "^13.0.0"}}`,
				"yarn.lock":      "# yarn lockfile v1",
			},
			expectedFramework:  "Next.js",
			expectedInstallCmd: "yarn install",
			expectedBuildCmd:   "yarn build",
		},
		{
			name: "Django with uv",
			files: map[string]string{
				"manage.py":      "#!/usr/bin/env python",
				"pyproject.toml": `[project]\ndependencies = ["Django>=4.2.0"]`,
				"uv.lock":        "version = 1",
			},
			expectedFramework:  "Django",
			expectedInstallCmd: "uv sync",
			expectedBuildCmd:   "python manage.py collectstatic --noinput",
		},
		{
			name: "Astro with bun",
			files: map[string]string{
				"astro.config.js":   "export default {}",
				"package.json":      `{"dependencies": {"astro": "^3.0.0"}}`,
				"bun.lockb":         "binary content",
				"src/pages/index.astro": "---\n---\n<h1>Astro</h1>",
				"public/favicon.ico": "favicon",
			},
			expectedFramework:  "Astro",
			expectedInstallCmd: "bun install",
			expectedBuildCmd:   "bun run build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			if len(detection.BuildPlan) < 2 {
				t.Fatalf("Expected at least 2 build commands, got %d", len(detection.BuildPlan))
			}

			if detection.BuildPlan[0] != tt.expectedInstallCmd {
				t.Errorf("Expected install command %s, got %s", tt.expectedInstallCmd, detection.BuildPlan[0])
			}

			found := false
			for _, cmd := range detection.BuildPlan {
				if cmd == tt.expectedBuildCmd {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected build command %s not found in %v", tt.expectedBuildCmd, detection.BuildPlan)
			}
		})
	}
}

func TestHealthCheckEndpoints(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		expectedFramework string
		expectedPath     string
		expectedTimeout  int
	}{
		{
			name: "Django health check",
			files: map[string]string{
				"manage.py":        "#!/usr/bin/env python",
				"requirements.txt": "Django==4.2.0",
			},
			expectedFramework: "Django",
			expectedPath:     "/healthz",
			expectedTimeout:  30,
		},
		{
			name: "FastAPI health check",
			files: map[string]string{
				"main.py":          "from fastapi import FastAPI\napp = FastAPI()",
				"requirements.txt": "fastapi==0.104.0",
			},
			expectedFramework: "FastAPI",
			expectedPath:     "/health",
			expectedTimeout:  30,
		},
		{
			name: "Rails health check",
			files: map[string]string{
				"bin/rails":             "#!/usr/bin/env ruby",
				"Gemfile.lock":          "GEM\nPLATFORMS\n  ruby",
				"config/application.rb": "require 'rails/all'",
			},
			expectedFramework: "Rails",
			expectedPath:     "/up",
			expectedTimeout:  30,
		},
		{
			name: "Go service health check",
			files: map[string]string{
				"go.mod":  "module myapp\ngo 1.21",
				"main.go": "package main\nfunc main() {}",
			},
			expectedFramework: "Go",
			expectedPath:     "/healthz",
			expectedTimeout:  30,
		},
		{
			name: "Spring Boot health check",
			files: map[string]string{
				"pom.xml": `<?xml version="1.0"?>
<project>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>`,
				"src/main/java/Application.java": "public class Application {}",
			},
			expectedFramework: "Spring Boot",
			expectedPath:     "/actuator/health",
			expectedTimeout:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.expectedFramework {
				t.Errorf("Expected framework %s, got %s", tt.expectedFramework, detection.Framework)
			}

			if detection.Healthcheck == nil {
				t.Fatal("Expected healthcheck to be configured")
			}

			if path, ok := detection.Healthcheck["path"].(string); ok {
				if path != tt.expectedPath {
					t.Errorf("Expected health check path %s, got %s", tt.expectedPath, path)
				}
			} else {
				t.Error("Expected health check path to be a string")
			}

			if timeout, ok := detection.Healthcheck["timeout_seconds"]; ok {
				// Handle both int and float64 types since JSON unmarshaling can produce either
				var timeoutInt int
				switch v := timeout.(type) {
				case int:
					timeoutInt = v
				case float64:
					timeoutInt = int(v)
				default:
					t.Errorf("Expected timeout_seconds to be a number, got %T", timeout)
					return
				}
				if timeoutInt != tt.expectedTimeout {
					t.Errorf("Expected timeout %d, got %d", tt.expectedTimeout, timeoutInt)
				}
			} else {
				t.Error("Expected timeout_seconds to be present")
			}

			if expect, ok := detection.Healthcheck["expect"]; ok {
				// Handle both int and float64 types since JSON unmarshaling can produce either
				var expectInt int
				switch v := expect.(type) {
				case int:
					expectInt = v
				case float64:
					expectInt = int(v)
				default:
					t.Errorf("Expected expect to be a number, got %T", expect)
					return
				}
				if expectInt != 200 {
					t.Errorf("Expected status code 200, got %d", expectInt)
				}
			} else {
				t.Error("Expected expect to be present")
			}
		})
	}
}