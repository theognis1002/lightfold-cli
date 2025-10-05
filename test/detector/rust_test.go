package detector_test

import (
	"testing"
)

// Rust Language and Framework Detection Tests

func TestRustLanguageDetection(t *testing.T) {
	tests := []struct {
		name             string
		files            map[string]string
		expectedLanguage string
	}{
		{
			name: "Rust project with Cargo.toml",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "myapp"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio = { version = "1.0", features = ["full"] }`,
				"src/main.rs": `fn main() {
    println!("Hello, world!");
}`,
			},
			expectedLanguage: "Rust",
		},
		{
			name: "Rust library project",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "mylib"
version = "0.1.0"

[dependencies]`,
				"src/lib.rs": `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}`,
			},
			expectedLanguage: "Rust",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Language != tt.expectedLanguage {
				t.Errorf("Expected language %s, got %s", tt.expectedLanguage, detection.Language)
			}
		})
	}
}

func TestActixWebDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		minConfidence     float64
	}{
		{
			name: "Actix-web with full setup",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "actix-web-app"
version = "0.1.0"
edition = "2021"

[dependencies]
actix-web = "4.0"
actix-rt = "2.0"
serde = { version = "1.0", features = ["derive"] }`,
				"src/main.rs": `use actix_web::{web, App, HttpServer, Responder};

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    HttpServer::new(|| {
        App::new()
            .route("/", web::get().to(index))
    })
    .bind("127.0.0.1:8080")?
    .run()
    .await
}

async fn index() -> impl Responder {
    "Hello World!"
}`,
			},
			expectedFramework: "Actix-web",
			expectedLanguage:  "Rust",
			expectedSignals:   []string{"Cargo.toml", "actix-web in Cargo.toml", ".rs files"},
			minConfidence:     0.7,
		},
		{
			name: "Actix-web minimal",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "app"
version = "0.1.0"

[dependencies]
actix-web = "4"`,
				"src/main.rs": `use actix_web::*;
fn main() {}`,
			},
			expectedFramework: "Actix-web",
			expectedLanguage:  "Rust",
			expectedSignals:   []string{"Cargo.toml", "actix-web in Cargo.toml", ".rs files"},
			minConfidence:     0.7,
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

			if detection.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %f, got %f", tt.minConfidence, detection.Confidence)
			}

			for _, expectedSignal := range tt.expectedSignals {
				found := false
				for _, signal := range detection.Signals {
					if signal == expectedSignal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}

			// Verify build output
			if detection.Meta["build_output"] != "target/release/" {
				t.Errorf("Expected build_output 'target/release/', got '%s'", detection.Meta["build_output"])
			}

			// Verify build command
			if len(detection.BuildPlan) == 0 || detection.BuildPlan[0] != "cargo build --release" {
				t.Errorf("Expected cargo build --release in build plan, got %v", detection.BuildPlan)
			}
		})
	}
}

func TestAxumDetection(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedFramework string
		expectedLanguage  string
		expectedSignals   []string
		minConfidence     float64
	}{
		{
			name: "Axum with Tokio",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "axum-app"
version = "0.1.0"
edition = "2021"

[dependencies]
axum = "0.6"
tokio = { version = "1.0", features = ["full"] }
tower = "0.4"`,
				"src/main.rs": `use axum::{
    routing::get,
    Router,
};

#[tokio::main]
async fn main() {
    let app = Router::new()
        .route("/", get(|| async { "Hello, World!" }));

    axum::Server::bind(&"0.0.0.0:3000".parse().unwrap())
        .serve(app.into_make_service())
        .await
        .unwrap();
}`,
			},
			expectedFramework: "Axum",
			expectedLanguage:  "Rust",
			expectedSignals:   []string{"Cargo.toml", "axum and tokio in Cargo.toml", ".rs files"},
			minConfidence:     0.7,
		},
		{
			name: "Axum with database",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "axum-db-app"
version = "0.1.0"

[dependencies]
axum = "0.6"
tokio = { version = "1", features = ["full"] }
sqlx = { version = "0.6", features = ["postgres"] }`,
				"src/main.rs": `use axum::*;
#[tokio::main]
async fn main() {}`,
			},
			expectedFramework: "Axum",
			expectedLanguage:  "Rust",
			expectedSignals:   []string{"Cargo.toml", "axum and tokio in Cargo.toml", ".rs files"},
			minConfidence:     0.7,
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

			if detection.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %f, got %f", tt.minConfidence, detection.Confidence)
			}

			for _, expectedSignal := range tt.expectedSignals {
				found := false
				for _, signal := range detection.Signals {
					if signal == expectedSignal {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected signal '%s' not found in %v", expectedSignal, detection.Signals)
				}
			}

			// Verify build output
			if detection.Meta["build_output"] != "target/release/" {
				t.Errorf("Expected build_output 'target/release/', got '%s'", detection.Meta["build_output"])
			}

			// Verify build command
			if len(detection.BuildPlan) == 0 || detection.BuildPlan[0] != "cargo build --release" {
				t.Errorf("Expected cargo build --release in build plan, got %v", detection.BuildPlan)
			}
		})
	}
}

func TestRustBuildCommands(t *testing.T) {
	tests := []struct {
		name         string
		framework    string
		files        map[string]string
		expectBuild  []string
		expectRunCmd string
	}{
		{
			name:      "Actix-web build and run",
			framework: "Actix-web",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "test-app"
version = "0.1.0"

[dependencies]
actix-web = "4.0"`,
				"src/main.rs": "use actix_web::*;",
			},
			expectBuild:  []string{"cargo build --release"},
			expectRunCmd: "./target/release/",
		},
		{
			name:      "Axum build and run",
			framework: "Axum",
			files: map[string]string{
				"Cargo.toml": `[package]
name = "axum-test"
version = "0.1.0"

[dependencies]
axum = "0.6"
tokio = "1.0"`,
				"src/main.rs": "use axum::*;",
			},
			expectBuild:  []string{"cargo build --release"},
			expectRunCmd: "./target/release/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath := createTestProject(t, tt.files)
			detection := captureDetectFramework(t, projectPath)

			if detection.Framework != tt.framework {
				t.Errorf("Expected framework %s, got %s", tt.framework, detection.Framework)
			}

			// Verify build commands
			for i, expectedCmd := range tt.expectBuild {
				if i >= len(detection.BuildPlan) {
					t.Errorf("Expected build command '%s' at index %d, but BuildPlan is too short", expectedCmd, i)
					continue
				}
				if detection.BuildPlan[i] != expectedCmd {
					t.Errorf("Expected build command '%s', got '%s'", expectedCmd, detection.BuildPlan[i])
				}
			}

			// Verify run command starts with expected path
			if len(detection.RunPlan) == 0 {
				t.Error("Expected non-empty run plan")
			} else if len(detection.RunPlan[0]) < len(tt.expectRunCmd) ||
				detection.RunPlan[0][:len(tt.expectRunCmd)] != tt.expectRunCmd {
				t.Errorf("Expected run command to start with '%s', got '%s'", tt.expectRunCmd, detection.RunPlan[0])
			}
		})
	}
}
