package installers

import (
	"fmt"
	"strings"

	"lightfold/pkg/runtime"
)

type dockerInstaller struct{}

func init() {
	Register(&dockerInstaller{})
}

func (d *dockerInstaller) Runtime() runtime.Runtime {
	return runtime.RuntimeDocker
}

func (d *dockerInstaller) IsInstalled(ctx *Context) (bool, error) {
	// Check if docker compose (V2) is available
	// Use sudo since deploy user might not be in docker group yet
	result := ctx.SSH.ExecuteSudo("docker compose version 2>/dev/null || echo 'not-found'")
	if result.Error != nil {
		return false, result.Error
	}

	output := strings.TrimSpace(result.Stdout)
	if output == "not-found" || output == "" {
		return false, nil
	}

	// Should contain "Docker Compose version" for V2
	if !strings.Contains(output, "Docker Compose version") {
		return false, nil
	}

	return true, nil
}

func (d *dockerInstaller) Install(ctx *Context) error {
	logOutput(ctx, "  Installing Docker with Compose V2...")

	// Remove old Docker packages that might conflict
	d.removeOldDocker(ctx)

	// Install prerequisites
	if err := d.installPrerequisites(ctx); err != nil {
		return err
	}

	// Add Docker's official GPG key and repository
	if err := d.addDockerRepository(ctx); err != nil {
		return err
	}

	// Install Docker Engine with Compose plugin
	if err := d.installDockerEngine(ctx); err != nil {
		return err
	}

	// Start and enable Docker service
	if err := d.enableDockerService(ctx); err != nil {
		return err
	}

	// Add deploy user to docker group
	if err := d.addUserToDockerGroup(ctx); err != nil {
		return err
	}

	// Verify installation
	if err := d.verifyInstallation(ctx); err != nil {
		return err
	}

	return nil
}

func (d *dockerInstaller) removeOldDocker(ctx *Context) {
	// Remove old Docker packages that use V1 compose syntax
	packages := []string{
		"docker.io",
		"docker-compose",
		"docker-doc",
		"podman-docker",
		"containerd",
		"runc",
	}

	for _, pkg := range packages {
		ctx.SSH.ExecuteSudo(fmt.Sprintf("apt-get remove -y %s 2>/dev/null || true", pkg))
	}

	ctx.SSH.ExecuteSudo("apt-get autoremove -y 2>/dev/null || true")
}

func (d *dockerInstaller) installPrerequisites(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo("apt-get update")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to update apt", result)
	}

	prereqs := "ca-certificates curl gnupg"
	result = ctx.SSH.ExecuteSudo(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", prereqs))
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install prerequisites", result)
	}

	return nil
}

func (d *dockerInstaller) addDockerRepository(ctx *Context) error {
	// Create keyrings directory
	result := ctx.SSH.ExecuteSudo("install -m 0755 -d /etc/apt/keyrings")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to create keyrings directory", result)
	}

	// Download Docker's GPG key
	result = ctx.SSH.ExecuteSudo("curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to download Docker GPG key", result)
	}

	result = ctx.SSH.ExecuteSudo("chmod a+r /etc/apt/keyrings/docker.asc")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to set GPG key permissions", result)
	}

	// Add Docker repository
	// Wrap in bash -c so the entire pipeline runs under sudo
	addRepoCmd := `bash -c 'echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" > /etc/apt/sources.list.d/docker.list'`
	result = ctx.SSH.ExecuteSudo(addRepoCmd)
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to add Docker repository", result)
	}

	// Update apt with new repository
	result = ctx.SSH.ExecuteSudo("apt-get update")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to update apt after adding Docker repo", result)
	}

	return nil
}

func (d *dockerInstaller) installDockerEngine(ctx *Context) error {
	packages := "docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin"
	result := ctx.SSH.ExecuteSudo(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y %s", packages))
	if ctx.Tail != nil {
		ctx.Tail(result, 5)
	}
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to install Docker Engine", result)
	}

	return nil
}

func (d *dockerInstaller) enableDockerService(ctx *Context) error {
	result := ctx.SSH.ExecuteSudo("systemctl enable docker")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to enable Docker service", result)
	}

	result = ctx.SSH.ExecuteSudo("systemctl start docker")
	if result.Error != nil || result.ExitCode != 0 {
		return formatCommandError("failed to start Docker service", result)
	}

	return nil
}

func (d *dockerInstaller) addUserToDockerGroup(ctx *Context) error {
	// Add deploy user to docker group so they can run docker without sudo
	result := ctx.SSH.ExecuteSudo("usermod -aG docker deploy")
	if result.Error != nil || result.ExitCode != 0 {
		// Non-fatal: user might not exist or might already be in group
		logOutput(ctx, "  Warning: could not add deploy user to docker group")
	}

	return nil
}

func (d *dockerInstaller) verifyInstallation(ctx *Context) error {
	// Verify docker is installed
	result := ctx.SSH.Execute("docker --version")
	dockerVersion := strings.TrimSpace(result.Stdout)
	if result.Error != nil || result.ExitCode != 0 || dockerVersion == "" {
		return formatCommandError("Docker installation verification failed", result)
	}

	// Verify docker compose V2 is installed
	result = ctx.SSH.Execute("docker compose version")
	composeVersion := strings.TrimSpace(result.Stdout)
	if result.Error != nil || result.ExitCode != 0 || composeVersion == "" {
		return formatCommandError("Docker Compose V2 installation verification failed", result)
	}

	if !strings.Contains(composeVersion, "Docker Compose version") {
		return fmt.Errorf("expected Docker Compose V2 but got: %s", composeVersion)
	}

	logOutput(ctx, fmt.Sprintf("  Docker installed: %s", dockerVersion))
	logOutput(ctx, fmt.Sprintf("  %s", composeVersion))

	return nil
}
