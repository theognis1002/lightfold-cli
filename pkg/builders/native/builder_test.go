package native

import (
	"context"
	"testing"

	"lightfold/pkg/builders"
	"lightfold/pkg/detector"
)

func TestNativeBuilder_Name(t *testing.T) {
	builder := &NativeBuilder{}
	if builder.Name() != "native" {
		t.Errorf("Expected name 'native', got '%s'", builder.Name())
	}
}

func TestNativeBuilder_IsAvailable(t *testing.T) {
	builder := &NativeBuilder{}
	if !builder.IsAvailable() {
		t.Error("Native builder should always be available")
	}
}

func TestNativeBuilder_NeedsNginx(t *testing.T) {
	builder := &NativeBuilder{}
	if !builder.NeedsNginx() {
		t.Error("Native builder should always need nginx")
	}
}

func TestNativeBuilder_Build_NoBuildPlan(t *testing.T) {
	builder := &NativeBuilder{}

	detection := &detector.Detection{
		Framework: "Unknown",
		Language:  "Unknown",
		BuildPlan: []string{}, // Empty build plan
	}

	result, err := builder.Build(context.Background(), &builders.BuildOptions{
		Detection: detection,
	})

	if err != nil {
		t.Fatalf("Expected no error for empty build plan, got: %v", err)
	}

	if !result.Success {
		t.Error("Expected success for empty build plan")
	}

	if result.StartCommand != "" {
		t.Errorf("Expected empty start command, got '%s'", result.StartCommand)
	}
}

func TestNativeBuilder_Build_NilDetection(t *testing.T) {
	builder := &NativeBuilder{}

	result, err := builder.Build(context.Background(), &builders.BuildOptions{
		Detection: nil,
	})

	if err != nil {
		t.Fatalf("Expected no error for nil detection, got: %v", err)
	}

	if !result.Success {
		t.Error("Expected success for nil detection")
	}
}

func TestGetAppName(t *testing.T) {
	tests := []struct {
		name        string
		releasePath string
		want        string
	}{
		{
			name:        "standard release path",
			releasePath: "/srv/myapp/releases/20240101120000",
			want:        "myapp",
		},
		{
			name:        "nested app path",
			releasePath: "/srv/my-cool-app/releases/20240101120000",
			want:        "my-cool-app",
		},
		{
			name:        "short path",
			releasePath: "/srv",
			want:        "app",
		},
		{
			name:        "empty path",
			releasePath: "",
			want:        "app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAppName(tt.releasePath)
			if got != tt.want {
				t.Errorf("getAppName(%q) = %q, want %q", tt.releasePath, got, tt.want)
			}
		})
	}
}

func TestAdjustBuildCommand_Python(t *testing.T) {
	detection := &detector.Detection{
		Language: "Python",
	}

	releasePath := "/srv/myapp/releases/20240101120000"

	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{
			name: "pip install command",
			cmd:  "pip install -r requirements.txt",
			want: "source /srv/myapp/shared/venv/bin/activate && pip install -r requirements.txt",
		},
		{
			name: "poetry install command",
			cmd:  "poetry install",
			want: "source /srv/myapp/shared/venv/bin/activate && poetry install",
		},
		{
			name: "uv install command",
			cmd:  "uv pip install -r requirements.txt",
			want: "source /srv/myapp/shared/venv/bin/activate && uv pip install -r requirements.txt",
		},
		{
			name: "non-install command",
			cmd:  "python manage.py migrate",
			want: "python manage.py migrate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustBuildCommand(tt.cmd, releasePath, detection)
			if got != tt.want {
				t.Errorf("adjustBuildCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAdjustBuildCommand_NonPython(t *testing.T) {
	detection := &detector.Detection{
		Language: "JavaScript",
	}

	releasePath := "/srv/myapp/releases/20240101120000"
	cmd := "npm install"

	got := adjustBuildCommand(cmd, releasePath, detection)
	if got != cmd {
		t.Errorf("adjustBuildCommand() = %q, want %q (should not modify)", got, cmd)
	}
}

func TestGetPackageManagerPath(t *testing.T) {
	tests := []struct {
		name           string
		packageManager string
		want           string
	}{
		{
			name:           "bun",
			packageManager: "bun",
			want:           "export PATH=$HOME/.bun/bin:$PATH && ",
		},
		{
			name:           "poetry",
			packageManager: "poetry",
			want:           "export PATH=$HOME/.local/bin:$PATH && ",
		},
		{
			name:           "uv",
			packageManager: "uv",
			want:           "export PATH=$HOME/.cargo/bin:$PATH && ",
		},
		{
			name:           "npm",
			packageManager: "npm",
			want:           "",
		},
		{
			name:           "yarn",
			packageManager: "yarn",
			want:           "",
		},
		{
			name:           "unknown",
			packageManager: "unknown",
			want:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := &detector.Detection{
				Meta: map[string]string{
					"package_manager": tt.packageManager,
				},
			}

			got := getPackageManagerPath(detection)
			if got != tt.want {
				t.Errorf("getPackageManagerPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPackageManagerPath_NilDetection(t *testing.T) {
	got := getPackageManagerPath(nil)
	if got != "" {
		t.Errorf("getPackageManagerPath(nil) = %q, want empty string", got)
	}
}

func TestGetPackageManagerPath_NoPackageManager(t *testing.T) {
	detection := &detector.Detection{
		Meta: map[string]string{},
	}

	got := getPackageManagerPath(detection)
	if got != "" {
		t.Errorf("getPackageManagerPath() = %q, want empty string", got)
	}
}
