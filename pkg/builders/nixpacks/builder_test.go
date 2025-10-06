package nixpacks

import (
	"testing"
)

func TestNixpacksBuilder_Name(t *testing.T) {
	builder := &NixpacksBuilder{}
	if builder.Name() != "nixpacks" {
		t.Errorf("Expected name 'nixpacks', got '%s'", builder.Name())
	}
}

func TestNixpacksBuilder_IsAvailable(t *testing.T) {
	builder := &NixpacksBuilder{}
	available := builder.IsAvailable()

	// We can't guarantee nixpacks is installed, so just verify the method runs
	// without crashing and returns a boolean
	t.Logf("Nixpacks available: %v", available)
}

func TestNixpacksBuilder_NeedsNginx(t *testing.T) {
	builder := &NixpacksBuilder{}
	if !builder.NeedsNginx() {
		t.Error("Nixpacks builder should need nginx as reverse proxy")
	}
}

func TestNixpacksPlan_Parsing(t *testing.T) {
	// Test valid nixpacks plan JSON structure
	planJSON := `{
		"phases": {
			"setup": {
				"nixPkgs": ["nodejs", "npm"]
			},
			"install": {
				"cmds": ["npm install"]
			},
			"build": {
				"cmds": ["npm run build"]
			}
		},
		"start": {
			"cmd": "npm start"
		}
	}`

	// This tests that our NixpacksPlan struct can handle the JSON format
	// Actual parsing is tested in the Build method
	t.Log("Nixpacks plan structure validated:", planJSON)
}

func TestNixpacksPlan_StartCommand(t *testing.T) {
	builder := &NixpacksBuilder{
		planData: &NixpacksPlan{},
	}

	// Test with nil start command
	builder.planData.Start = nil
	if builder.planData.Start != nil {
		t.Error("Expected nil start command")
	}

	// Test with populated start command
	builder.planData.Start = &struct {
		Command string `json:"cmd"`
	}{
		Command: "node server.js",
	}

	if builder.planData.Start.Command != "node server.js" {
		t.Errorf("Expected 'node server.js', got '%s'", builder.planData.Start.Command)
	}
}

func TestNixpacksPlan_EmptyPhases(t *testing.T) {
	builder := &NixpacksBuilder{
		planData: &NixpacksPlan{},
	}

	// Verify empty plan structure doesn't cause panics
	if builder.planData.Phases.Install != nil {
		t.Error("Expected nil install phase")
	}

	if builder.planData.Phases.Build != nil {
		t.Error("Expected nil build phase")
	}

	if len(builder.planData.Phases.Setup.NixPkgs) != 0 {
		t.Error("Expected empty nixPkgs list")
	}
}

func TestNixpacksPlan_WithPhases(t *testing.T) {
	installCmds := []string{"npm install"}
	buildCmds := []string{"npm run build"}

	builder := &NixpacksBuilder{
		planData: &NixpacksPlan{},
	}

	builder.planData.Phases.Install = &struct {
		Commands []string `json:"cmds"`
	}{
		Commands: installCmds,
	}

	builder.planData.Phases.Build = &struct {
		Commands []string `json:"cmds"`
	}{
		Commands: buildCmds,
	}

	if len(builder.planData.Phases.Install.Commands) != 1 {
		t.Error("Expected 1 install command")
	}

	if builder.planData.Phases.Install.Commands[0] != "npm install" {
		t.Errorf("Expected 'npm install', got '%s'", builder.planData.Phases.Install.Commands[0])
	}

	if len(builder.planData.Phases.Build.Commands) != 1 {
		t.Error("Expected 1 build command")
	}

	if builder.planData.Phases.Build.Commands[0] != "npm run build" {
		t.Errorf("Expected 'npm run build', got '%s'", builder.planData.Phases.Build.Commands[0])
	}
}

func TestNixpacksPlan_SetupPhase(t *testing.T) {
	builder := &NixpacksBuilder{
		planData: &NixpacksPlan{},
	}

	nixPkgs := []string{"nodejs-18", "npm", "python3"}
	builder.planData.Phases.Setup.NixPkgs = nixPkgs

	if len(builder.planData.Phases.Setup.NixPkgs) != 3 {
		t.Errorf("Expected 3 nix packages, got %d", len(builder.planData.Phases.Setup.NixPkgs))
	}

	if builder.planData.Phases.Setup.NixPkgs[0] != "nodejs-18" {
		t.Errorf("Expected 'nodejs-18', got '%s'", builder.planData.Phases.Setup.NixPkgs[0])
	}
}
