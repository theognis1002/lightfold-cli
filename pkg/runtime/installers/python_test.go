package installers

import (
	"lightfold/pkg/detector"
	"lightfold/pkg/runtime"
	"strings"
	"testing"
)

func TestPythonInstaller_Runtime(t *testing.T) {
	installer := &pythonInstaller{}
	if installer.Runtime() != runtime.RuntimePython {
		t.Errorf("Expected runtime %q, got %q", runtime.RuntimePython, installer.Runtime())
	}
}

func TestPythonInstaller_IsInstalled_NotInstalled(t *testing.T) {
	mockSSH := newMockSSHExecutor()
	mockSSH.failures["python3 --version 2>/dev/null || echo 'not-found'"] = true

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	installed, err := installer.IsInstalled(ctx)
	if err != nil {
		t.Fatalf("IsInstalled returned error: %v", err)
	}

	if installed {
		t.Error("Expected Python to not be installed, but IsInstalled returned true")
	}
}

func TestPythonInstaller_IsInstalled_WithPython3(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	installed, err := installer.IsInstalled(ctx)
	if err != nil {
		t.Fatalf("IsInstalled returned error: %v", err)
	}

	if !installed {
		t.Error("Expected Python to be installed, but IsInstalled returned false")
	}
}

func TestPythonInstaller_Install_CreatesSymlinks(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify python3/pip3 packages were installed
	if !mockSSH.hasCommand("apt-get install") || !mockSSH.hasCommand("python3") {
		t.Error("Expected apt-get install command for python3")
	}

	// CRITICAL: Verify symlinks were created
	if !mockSSH.hasCommand("ln -sf /usr/bin/python3 /usr/bin/python") {
		t.Error("Expected symlink creation for python -> python3")
	}

	if !mockSSH.hasCommand("ln -sf /usr/bin/pip3 /usr/bin/pip") {
		t.Error("Expected symlink creation for pip -> pip3")
	}

	// CRITICAL: Verify symlinks were validated
	if !mockSSH.hasCommand("python --version") {
		t.Error("Expected python symlink verification command")
	}
}

func TestPythonInstaller_Install_SymlinkCreationFailure(t *testing.T) {
	mockSSH := newMockSSHExecutor()
	// Simulate symlink creation failure
	mockSSH.failures["sudo -n ln -sf /usr/bin/python3 /usr/bin/python"] = true

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	err := installer.Install(ctx)
	if err == nil {
		t.Fatal("Expected Install to fail when symlink creation fails, but it succeeded")
	}

	if !strings.Contains(err.Error(), "python symlink") {
		t.Errorf("Expected error message about python symlink, got: %v", err)
	}
}

func TestPythonInstaller_Install_SymlinkVerificationFailure(t *testing.T) {
	mockSSH := newMockSSHExecutor()
	// Simulate symlink verification failure (python command doesn't work)
	mockSSH.failures["python --version"] = true

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	err := installer.Install(ctx)
	if err == nil {
		t.Fatal("Expected Install to fail when symlink verification fails, but it succeeded")
	}

	if !strings.Contains(err.Error(), "verification failed") {
		t.Errorf("Expected error message about verification failure, got: %v", err)
	}
}

func TestPythonInstaller_Install_WithPoetry(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
		Detection: &detector.Detection{
			Meta: map[string]string{
				"package_manager": "poetry",
			},
		},
	}

	err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify poetry installation
	if !mockSSH.hasCommand("install.python-poetry.org") {
		t.Error("Expected poetry installation command")
	}
}

func TestPythonInstaller_Install_WithPipenv(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
		Detection: &detector.Detection{
			Meta: map[string]string{
				"package_manager": "pipenv",
			},
		},
	}

	err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify pipenv installation
	if !mockSSH.hasCommand("pip3 install --user pipenv") {
		t.Error("Expected pipenv installation command")
	}
}

func TestPythonInstaller_Install_WithUv(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
		Detection: &detector.Detection{
			Meta: map[string]string{
				"package_manager": "uv",
			},
		},
	}

	err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify uv installation
	if !mockSSH.hasCommand("astral.sh/uv/install.sh") {
		t.Error("Expected uv installation command")
	}
}

func TestPythonInstaller_IsInstalled_WithPoetry(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
		Detection: &detector.Detection{
			Meta: map[string]string{
				"package_manager": "poetry",
			},
		},
	}

	installed, err := installer.IsInstalled(ctx)
	if err != nil {
		t.Fatalf("IsInstalled returned error: %v", err)
	}

	// Should check for both python3 and poetry
	if !installed {
		t.Error("Expected Python with Poetry to be installed")
	}

	if !mockSSH.hasCommand("command -v poetry") {
		t.Error("Expected poetry availability check")
	}
}

// Regression test for the nixpacks "python: command not found" bug
func TestPythonInstaller_Install_NixpacksCompatibility(t *testing.T) {
	mockSSH := newMockSSHExecutor()

	installer := &pythonInstaller{}
	ctx := &Context{
		SSH: mockSSH,
	}

	err := installer.Install(ctx)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify execution order: install packages -> create symlinks -> verify
	var aptIndex, pythonSymlinkIndex, pipSymlinkIndex, verifyIndex int

	for i, cmd := range mockSSH.commands {
		switch {
		case strings.Contains(cmd, "apt-get install"):
			aptIndex = i
		case strings.Contains(cmd, "ln -sf /usr/bin/python3 /usr/bin/python"):
			pythonSymlinkIndex = i
		case strings.Contains(cmd, "ln -sf /usr/bin/pip3 /usr/bin/pip"):
			pipSymlinkIndex = i
		case strings.Contains(cmd, "python --version"):
			verifyIndex = i
		}
	}

	// Verify correct ordering
	if aptIndex >= pythonSymlinkIndex {
		t.Error("apt-get install should run before python symlink creation")
	}

	if pythonSymlinkIndex >= pipSymlinkIndex {
		t.Error("python symlink should be created before pip symlink")
	}

	if pipSymlinkIndex >= verifyIndex {
		t.Error("symlink verification should run after all symlinks are created")
	}

	// CRITICAL: Ensure symlinks are created and verified
	if mockSSH.commandCount("ln -sf") != 2 {
		t.Errorf("Expected 2 symlink creation commands, got %d", mockSSH.commandCount("ln -sf"))
	}

	if mockSSH.commandCount("python --version") == 0 {
		t.Error("Expected python verification command to ensure symlink works")
	}
}
