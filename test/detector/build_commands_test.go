package detector_test

import (
	"strings"
	"testing"
)

// TestGoBuildCommand verifies that Go build command is valid
func TestGoBuildCommand(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"go.mod":  "module example.com/myapp\ngo 1.21",
		"main.go": "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}",
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Go" {
		t.Errorf("Expected Go, got %s", detection.Framework)
	}

	// Verify build command doesn't use invalid ./... syntax
	if len(detection.BuildPlan) == 0 {
		t.Fatal("Expected build plan to have at least one command")
	}

	buildCmd := detection.BuildPlan[0]
	if strings.Contains(buildCmd, "./...") {
		t.Errorf("Build command should not contain ./... with -o flag. Got: %s", buildCmd)
	}

	// Verify it uses the correct syntax
	if !strings.Contains(buildCmd, "go build -o app .") {
		t.Errorf("Expected 'go build -o app .', got: %s", buildCmd)
	}
}

// TestSpringBootMavenBuildOutput verifies correct jar path for Maven
func TestSpringBootMavenBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.0.0</version>
    </parent>
    <artifactId>my-spring-app</artifactId>
</project>`,
		"src/main/java/Application.java": `package com.example;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
}`,
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Spring Boot" {
		t.Errorf("Expected Spring Boot, got %s", detection.Framework)
	}

	// Verify build uses Maven
	if len(detection.BuildPlan) == 0 {
		t.Fatal("Expected build plan to have at least one command")
	}
	if !strings.Contains(detection.BuildPlan[0], "mvnw") {
		t.Errorf("Expected Maven wrapper in build command, got: %s", detection.BuildPlan[0])
	}

	// Verify run command uses target/ directory for Maven
	if len(detection.RunPlan) == 0 {
		t.Fatal("Expected run plan to have at least one command")
	}
	runCmd := detection.RunPlan[0]
	if !strings.Contains(runCmd, "target/*.jar") {
		t.Errorf("Expected Maven output path 'target/*.jar', got: %s", runCmd)
	}

	// Verify build_tool metadata
	if detection.Meta["build_tool"] != "maven" {
		t.Errorf("Expected build_tool=maven in metadata, got: %s", detection.Meta["build_tool"])
	}
}

// TestSpringBootGradleBuildOutput verifies correct jar path for Gradle
func TestSpringBootGradleBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"build.gradle": `plugins {
    id 'org.springframework.boot' version '3.0.0'
    id 'java'
}

group = 'com.example'
version = '0.0.1-SNAPSHOT'
sourceCompatibility = '17'

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web'
}`,
		"src/main/java/Application.java": `package com.example;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
}`,
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "Spring Boot" {
		t.Errorf("Expected Spring Boot, got %s", detection.Framework)
	}

	// Verify build uses Gradle
	if len(detection.BuildPlan) == 0 {
		t.Fatal("Expected build plan to have at least one command")
	}
	if !strings.Contains(detection.BuildPlan[0], "gradlew") {
		t.Errorf("Expected Gradle wrapper in build command, got: %s", detection.BuildPlan[0])
	}

	// Verify run command uses build/libs/ directory for Gradle
	if len(detection.RunPlan) == 0 {
		t.Fatal("Expected run plan to have at least one command")
	}
	runCmd := detection.RunPlan[0]
	if !strings.Contains(runCmd, "build/libs/*.jar") {
		t.Errorf("Expected Gradle output path 'build/libs/*.jar', got: %s", runCmd)
	}

	// Verify build_tool metadata
	if detection.Meta["build_tool"] != "gradle" {
		t.Errorf("Expected build_tool=gradle in metadata, got: %s", detection.Meta["build_tool"])
	}
}

// TestAspNetBuildOutput verifies correct dll execution command
func TestAspNetBuildOutput(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"MyApp.csproj": `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>
</Project>`,
		"Program.cs": `using Microsoft.AspNetCore.Builder;

var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "Hello World!");
app.Run();`,
	})

	detection := captureDetectFramework(t, projectPath)

	if detection.Framework != "ASP.NET Core" {
		t.Errorf("Expected ASP.NET Core, got %s", detection.Framework)
	}

	// Verify build includes publish step
	found := false
	for _, cmd := range detection.BuildPlan {
		if strings.Contains(cmd, "dotnet publish") && strings.Contains(cmd, "-o out") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'dotnet publish -o out' in build plan, got: %v", detection.BuildPlan)
	}

	// Verify run command uses shell expansion to find dll
	if len(detection.RunPlan) == 0 {
		t.Fatal("Expected run plan to have at least one command")
	}
	runCmd := detection.RunPlan[0]

	// Should not use wildcards directly, should use shell command
	if strings.Contains(runCmd, "dotnet out/*.dll") && !strings.Contains(runCmd, "$") {
		t.Errorf("Run command should use shell expansion, not direct wildcards. Got: %s", runCmd)
	}

	// Verify it uses ls or find to locate the dll
	if !strings.Contains(runCmd, "ls") && !strings.Contains(runCmd, "find") {
		t.Errorf("Expected shell command to find dll (ls or find), got: %s", runCmd)
	}
}

// TestExpressBuildCommands verifies no duplicate commands
func TestExpressBuildCommands(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		expectBuild   bool
	}{
		{
			name: "Express with npm",
			files: map[string]string{
				"package.json": `{
  "name": "express-app",
  "dependencies": {
    "express": "^4.18.0"
  },
  "scripts": {
    "start": "node server.js"
  }
}`,
				"server.js": `const express = require('express');
const app = express();
app.listen(3000);`,
			},
			expectBuild: false, // Express doesn't typically need a build step
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Express.js" {
				t.Errorf("Expected Express.js, got %s", detection.Framework)
			}

			// Verify build plan only has install command, no build command
			for _, cmd := range detection.BuildPlan {
				if strings.Contains(cmd, "build") && !strings.Contains(cmd, "install") {
					if !tt.expectBuild {
						t.Errorf("Unexpected build command in Express plan: %s", cmd)
					}
				}
			}

			// Verify run plan has only one command
			if len(detection.RunPlan) > 1 {
				t.Errorf("Expected single run command, got %d: %v", len(detection.RunPlan), detection.RunPlan)
			}

			// Verify run command uses start script
			if len(detection.RunPlan) > 0 {
				runCmd := detection.RunPlan[0]
				if !strings.Contains(runCmd, "start") {
					t.Errorf("Expected run command to use 'start' script, got: %s", runCmd)
				}
			}
		})
	}
}
