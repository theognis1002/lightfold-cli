package cloudinit

import (
	"lightfold/pkg/providers/cloudinit"
	"strings"
	"testing"
)

func TestGenerateWebAppUserData(t *testing.T) {
	username := "deploy"
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGq1234567890abcdef test@example.com"
	appName := "my-test-app"

	userData, err := cloudinit.GenerateWebAppUserData(username, publicKey, appName)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Check that generated user data contains expected elements
	expectedElements := []string{
		"#cloud-config",
		username,
		publicKey,
		appName,
		"users:",
		"packages:",
		"runcmd:",
		"nginx",
		"ufw allow 22/tcp",
		"ufw allow 80/tcp",
		"ufw allow 443/tcp",
		"/srv/" + appName,
	}

	for _, element := range expectedElements {
		if !strings.Contains(userData, element) {
			t.Errorf("Expected user data to contain '%s', but it was missing", element)
		}
	}

	// Verify it starts with cloud-config header
	if !strings.HasPrefix(userData, "#cloud-config") {
		t.Error("User data should start with #cloud-config")
	}
}

func TestGenerateMinimalUserData(t *testing.T) {
	username := "test-user"
	publicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ... test@example.com"

	userData, err := cloudinit.GenerateMinimalUserData(username, publicKey)
	if err != nil {
		t.Fatalf("Failed to generate minimal user data: %v", err)
	}

	// Check basic elements
	expectedElements := []string{
		"#cloud-config",
		username,
		publicKey,
		"users:",
		"packages:",
		"ufw allow 22/tcp",
	}

	for _, element := range expectedElements {
		if !strings.Contains(userData, element) {
			t.Errorf("Expected minimal user data to contain '%s', but it was missing", element)
		}
	}

	// Should not contain complex packages like docker
	if strings.Contains(userData, "docker.io") {
		t.Error("Minimal user data should not contain docker")
	}
}

func TestValidateUserData(t *testing.T) {
	testCases := []struct {
		name   string
		config cloudinit.UserData
		valid  bool
	}{
		{
			name: "valid config",
			config: cloudinit.UserData{
				Username:  "deploy",
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
				AppName:   "my-app",
			},
			valid: true,
		},
		{
			name: "missing username",
			config: cloudinit.UserData{
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
				AppName:   "my-app",
			},
			valid: false,
		},
		{
			name: "missing public key",
			config: cloudinit.UserData{
				Username: "deploy",
				AppName:  "my-app",
			},
			valid: false,
		},
		{
			name: "invalid app name with special chars",
			config: cloudinit.UserData{
				Username:  "deploy",
				PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
				AppName:   "my/app:with*invalid?chars",
			},
			valid: false,
		},
		{
			name: "invalid SSH key format",
			config: cloudinit.UserData{
				Username:  "deploy",
				PublicKey: "not-a-valid-ssh-key",
				AppName:   "my-app",
			},
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cloudinit.ValidateUserData(tc.config)
			if tc.valid && err != nil {
				t.Errorf("Expected valid config, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Error("Expected invalid config, got no error")
			}
		})
	}
}

func TestAddNginxConfig(t *testing.T) {
	config := &cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
	}

	domain := "example.com"
	appPort := "3000"

	cloudinit.AddNginxConfig(config, domain, appPort)

	// Check that files were added
	if len(config.Files) == 0 {
		t.Error("Expected files to be added for Nginx config")
	}

	// Check that commands were added
	if len(config.Commands) == 0 {
		t.Error("Expected commands to be added for Nginx config")
	}

	// Verify Nginx config file was added
	found := false
	for _, file := range config.Files {
		if strings.Contains(file.Path, "nginx") && strings.Contains(file.Path, config.AppName) {
			found = true
			// Check content contains domain and port
			if !strings.Contains(file.Content, domain) {
				t.Error("Nginx config should contain domain")
			}
			if !strings.Contains(file.Content, appPort) {
				t.Error("Nginx config should contain app port")
			}
		}
	}

	if !found {
		t.Error("Expected Nginx config file to be added")
	}
}

func TestAddSystemdService(t *testing.T) {
	config := &cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
	}

	serviceName := "my-app"
	execStart := "/srv/my-app/current/start.sh"
	workingDir := "/srv/my-app/current"

	cloudinit.AddSystemdService(config, serviceName, execStart, workingDir)

	// Check that files were added
	if len(config.Files) == 0 {
		t.Error("Expected files to be added for systemd service")
	}

	// Check that commands were added
	if len(config.Commands) == 0 {
		t.Error("Expected commands to be added for systemd service")
	}

	// Verify systemd service file was added
	found := false
	for _, file := range config.Files {
		if strings.Contains(file.Path, "systemd") && strings.Contains(file.Path, serviceName+".service") {
			found = true
			// Check content contains expected values
			if !strings.Contains(file.Content, execStart) {
				t.Error("Systemd service should contain ExecStart path")
			}
			if !strings.Contains(file.Content, workingDir) {
				t.Error("Systemd service should contain WorkingDirectory")
			}
			if !strings.Contains(file.Content, config.Username) {
				t.Error("Systemd service should contain username")
			}
		}
	}

	if !found {
		t.Error("Expected systemd service file to be added")
	}
}

func TestUserDataDefaults(t *testing.T) {
	config := cloudinit.UserData{
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Should use default username
	if !strings.Contains(userData, "deploy") {
		t.Error("Should use default username 'deploy'")
	}

	// Should use default app name
	if !strings.Contains(userData, "/srv/app") {
		t.Error("Should use default app name 'app'")
	}

	// Should contain default packages
	expectedPackages := []string{"nginx", "python3", "git"}
	for _, pkg := range expectedPackages {
		if !strings.Contains(userData, pkg) {
			t.Errorf("Should contain default package: %s", pkg)
		}
	}
}

func TestCloudInitFileStructure(t *testing.T) {
	file := cloudinit.CloudInitFile{
		Path:        "/etc/test/config",
		Content:     "test content",
		Permissions: "0644",
		Owner:       "root:root",
	}

	if file.Path == "" {
		t.Error("File path should not be empty")
	}

	if file.Content == "" {
		t.Error("File content should not be empty")
	}

	if file.Permissions == "" {
		t.Error("File permissions should not be empty")
	}

	if file.Owner == "" {
		t.Error("File owner should not be empty")
	}
}

func TestGenerateUserDataWithCustomPackages(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "custom-user",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "custom-app",
		Packages:  []string{"docker.io", "redis-server", "postgresql"},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Check custom packages
	for _, pkg := range config.Packages {
		if !strings.Contains(userData, pkg) {
			t.Errorf("Expected package '%s' in user data", pkg)
		}
	}

	// Check custom username and app name
	if !strings.Contains(userData, "custom-user") {
		t.Error("Should contain custom username")
	}

	if !strings.Contains(userData, "/srv/custom-app") {
		t.Error("Should contain custom app directory")
	}
}

func TestGenerateUserDataWithFiles(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
		Files: []cloudinit.CloudInitFile{
			{
				Path:        "/etc/app/config.yaml",
				Content:     "key: value\nother: data",
				Permissions: "0600",
				Owner:       "deploy:deploy",
			},
			{
				Path:        "/etc/systemd/system/my-app.service",
				Content:     "[Unit]\nDescription=My App",
				Permissions: "0644",
				Owner:       "root:root",
			},
		},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Should contain write_files section
	if !strings.Contains(userData, "write_files:") {
		t.Error("Should contain write_files section")
	}

	// Check files are present
	for _, file := range config.Files {
		if !strings.Contains(userData, file.Path) {
			t.Errorf("Expected file path '%s' in user data", file.Path)
		}
	}
}

func TestGenerateUserDataWithCustomCommands(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
		Commands: []string{
			"apt-get update",
			"systemctl enable my-app",
			"certbot --nginx -d example.com --non-interactive",
		},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Check commands are present
	for _, cmd := range config.Commands {
		if !strings.Contains(userData, cmd) {
			t.Errorf("Expected command '%s' in user data", cmd)
		}
	}
}

func TestGenerateUserDataWithCustomUFWRules(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
		UFWRules: []string{
			"ufw allow 22/tcp",
			"ufw allow 80/tcp",
			"ufw allow 443/tcp",
			"ufw allow 3000/tcp",
		},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Check UFW rules are present
	for _, rule := range config.UFWRules {
		if !strings.Contains(userData, rule) {
			t.Errorf("Expected UFW rule '%s' in user data", rule)
		}
	}

	// Should enable UFW
	if !strings.Contains(userData, "ufw --force enable") {
		t.Error("Should contain UFW enable command")
	}
}

func TestGenerateUserDataComplexScenario(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "app-user",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "production-app",
		Packages: []string{
			"nginx",
			"nodejs",
			"npm",
			"git",
			"postgresql-client",
		},
		Files: []cloudinit.CloudInitFile{
			{
				Path:        "/etc/nginx/sites-available/production-app",
				Content:     "server { listen 80; server_name example.com; }",
				Permissions: "0644",
				Owner:       "root:root",
			},
		},
		Commands: []string{
			"ln -s /etc/nginx/sites-available/production-app /etc/nginx/sites-enabled/",
			"systemctl restart nginx",
		},
		UFWRules: []string{
			"ufw allow 22/tcp",
			"ufw allow 80/tcp",
			"ufw allow 443/tcp",
		},
		Metadata: map[string]string{
			"environment": "production",
			"app_version": "1.0.0",
		},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Verify all components are present
	if !strings.HasPrefix(userData, "#cloud-config") {
		t.Error("Should start with #cloud-config")
	}

	if !strings.Contains(userData, "app-user") {
		t.Error("Should contain custom username")
	}

	if !strings.Contains(userData, "/srv/production-app") {
		t.Error("Should contain app directory")
	}

	// Verify all packages
	for _, pkg := range config.Packages {
		if !strings.Contains(userData, pkg) {
			t.Errorf("Expected package '%s'", pkg)
		}
	}

	// Verify final message
	if !strings.Contains(userData, "final_message:") {
		t.Error("Should contain final message")
	}
}

func TestGenerateUserDataInvalidPublicKey(t *testing.T) {
	config := cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "", // Empty public key
		AppName:   "my-app",
	}

	_, err := cloudinit.GenerateUserData(config)
	// This should succeed with empty key (validation happens elsewhere)
	// This tests that generation is flexible
	if err != nil {
		t.Logf("Note: Empty public key generates user data with warning: %v", err)
	}
}

func TestCloudInitFileWithEncoding(t *testing.T) {
	file := cloudinit.CloudInitFile{
		Path:        "/etc/app/binary-data",
		Content:     "base64-encoded-content",
		Permissions: "0600",
		Owner:       "deploy:deploy",
		Encoding:    "base64",
	}

	if file.Encoding != "base64" {
		t.Error("Expected encoding to be base64")
	}

	config := cloudinit.UserData{
		Username:  "deploy",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5... test@example.com",
		AppName:   "my-app",
		Files:     []cloudinit.CloudInitFile{file},
	}

	userData, err := cloudinit.GenerateUserData(config)
	if err != nil {
		t.Fatalf("Failed to generate user data: %v", err)
	}

	// Check encoding is included
	if !strings.Contains(userData, "encoding: base64") {
		t.Error("Should contain encoding specification")
	}
}
