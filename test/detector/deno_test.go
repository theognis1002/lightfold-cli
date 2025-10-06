package detector_test

import (
	"testing"
)

// Deno Runtime Detection Tests

func TestDenoWithExpress(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		expectedRuntime  string
		expectDenoInMeta bool
	}{
		{
			name: "Express with deno.json",
			files: map[string]string{
				"deno.json": `{
  "tasks": {
    "start": "deno run --allow-net main.ts"
  },
  "imports": {
    "express": "npm:express@^4.18.0"
  }
}`,
				"main.ts": `import express from "express";

const app = express();
const port = 3000;

app.get("/", (req, res) => {
  res.send("Hello from Deno!");
});

app.listen(port, () => {
  console.log("Server running at http://localhost:" + port);
});`,
				"package.json": `{
  "name": "deno-express",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
			},
			expectedRuntime:  "deno",
			expectDenoInMeta: true,
		},
		{
			name: "Express with deno.jsonc",
			files: map[string]string{
				"deno.jsonc": `{
  // Deno config with comments
  "tasks": {
    "start": "deno run --allow-net main.ts"
  }
}`,
				"main.ts": `import express from "npm:express";
const app = express();`,
				"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
			},
			expectedRuntime:  "deno",
			expectDenoInMeta: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Express.js" {
				t.Errorf("Expected framework Express.js, got %s", detection.Framework)
			}

			if tt.expectDenoInMeta {
				if runtime, ok := detection.Meta["runtime"]; ok {
					if runtime != tt.expectedRuntime {
						t.Errorf("Expected runtime %s in meta, got %s", tt.expectedRuntime, runtime)
					}
				} else {
					t.Error("Expected runtime in meta, but it was not found")
				}
			}

			// Verify Deno-specific build commands
			if len(detection.BuildPlan) > 0 {
				if detection.BuildPlan[0] != "deno cache main.ts" {
					t.Errorf("Expected Deno cache command, got %s", detection.BuildPlan[0])
				}
			}

			// Verify Deno-specific run commands
			if len(detection.RunPlan) > 0 {
				if detection.RunPlan[0] != "deno run --allow-net --allow-read --allow-env main.ts" {
					t.Errorf("Expected Deno run command, got %s", detection.RunPlan[0])
				}
			}
		})
	}
}

func TestDenoWithFastify(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		expectedRuntime  string
		expectDenoInMeta bool
	}{
		{
			name: "Fastify with deno.json",
			files: map[string]string{
				"deno.json": `{
  "tasks": {
    "start": "deno run --allow-net main.ts"
  },
  "imports": {
    "fastify": "npm:fastify@^4.0.0"
  }
}`,
				"main.ts": `import Fastify from "fastify";

const fastify = Fastify({
  logger: true
});

fastify.get("/", async (request, reply) => {
  return { hello: "world" };
});

const start = async () => {
  try {
    await fastify.listen({ port: 3000 });
  } catch (err) {
    fastify.log.error(err);
    Deno.exit(1);
  }
};

start();`,
				"package.json": `{
  "name": "deno-fastify",
  "dependencies": {
    "fastify": "^4.0.0"
  }
}`,
			},
			expectedRuntime:  "deno",
			expectDenoInMeta: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != "Fastify" {
				t.Errorf("Expected framework Fastify, got %s", detection.Framework)
			}

			if tt.expectDenoInMeta {
				if runtime, ok := detection.Meta["runtime"]; ok {
					if runtime != tt.expectedRuntime {
						t.Errorf("Expected runtime %s in meta, got %s", tt.expectedRuntime, runtime)
					}
				} else {
					t.Error("Expected runtime in meta, but it was not found")
				}
			}

			// Verify Deno-specific commands
			if len(detection.BuildPlan) > 0 && detection.BuildPlan[0] != "deno cache main.ts" {
				t.Errorf("Expected Deno cache command, got %s", detection.BuildPlan[0])
			}
		})
	}
}

func TestNodeJSWithoutDeno(t *testing.T) {
	tests := []struct {
		name            string
		files           map[string]string
		expectedRuntime string
		shouldHavePM    bool
	}{
		{
			name: "Express with npm (no Deno)",
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
const app = express();`,
				"package-lock.json": "{}",
			},
			expectedRuntime: "",
			shouldHavePM:    true,
		},
		{
			name: "Fastify with pnpm (no Deno)",
			files: map[string]string{
				"package.json": `{
  "name": "fastify-app",
  "dependencies": {
    "fastify": "^4.0.0"
  }
}`,
				"server.js":      `const fastify = require('fastify')();`,
				"pnpm-lock.yaml": "lockfileVersion: 5.4",
			},
			expectedRuntime: "",
			shouldHavePM:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			// Should NOT have deno runtime in meta
			if runtime, ok := detection.Meta["runtime"]; ok && runtime == "deno" {
				t.Error("Did not expect Deno runtime, but found it in meta")
			}

			// Should have package manager
			if tt.shouldHavePM {
				if _, ok := detection.Meta["package_manager"]; !ok {
					t.Error("Expected package_manager in meta for Node.js project")
				}
			}

			// Should NOT have Deno commands
			if len(detection.BuildPlan) > 0 {
				for _, cmd := range detection.BuildPlan {
					if len(cmd) >= 4 && cmd[:4] == "deno" {
						t.Errorf("Did not expect Deno command, got %s", cmd)
					}
				}
			}

			if len(detection.RunPlan) > 0 {
				for _, cmd := range detection.RunPlan {
					if len(cmd) >= 4 && cmd[:4] == "deno" {
						t.Errorf("Did not expect Deno command, got %s", cmd)
					}
				}
			}
		})
	}
}

func TestDenoBuildCommands(t *testing.T) {
	projectPath := createTestProject(t, map[string]string{
		"deno.json": `{
  "tasks": {
    "start": "deno run --allow-net main.ts"
  }
}`,
		"main.ts": `import express from "npm:express";
const app = express();
app.listen(3000);`,
		"package.json": `{"dependencies": {"express": "^4.0.0"}}`,
	})

	detection := captureDetectFramework(t, projectPath)

	// Verify Deno-specific commands
	expectedBuild := "deno cache main.ts"
	expectedRun := "deno run --allow-net --allow-read --allow-env main.ts"

	if len(detection.BuildPlan) == 0 {
		t.Error("Expected non-empty build plan for Deno project")
	} else if detection.BuildPlan[0] != expectedBuild {
		t.Errorf("Expected build command '%s', got '%s'", expectedBuild, detection.BuildPlan[0])
	}

	if len(detection.RunPlan) == 0 {
		t.Error("Expected non-empty run plan for Deno project")
	} else if detection.RunPlan[0] != expectedRun {
		t.Errorf("Expected run command '%s', got '%s'", expectedRun, detection.RunPlan[0])
	}
}
