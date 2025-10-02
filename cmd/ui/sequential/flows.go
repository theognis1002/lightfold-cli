package sequential

import (
	"fmt"
	"lightfold/pkg/config"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// CreateDigitalOceanFlow creates a sequential flow for DigitalOcean configuration
func CreateDigitalOceanFlow(projectName string) *FlowModel {
	steps := []Step{
		CreateIPStep("ip", "192.168.1.100"),
		CreateSSHKeyStep("ssh_key"),
		CreateUsernameStep("username", "root"),
	}

	flow := NewFlow("Configure DigitalOcean Deployment", steps)
	flow.SetProjectName(projectName)
	return flow
}

// CreateS3Flow creates a sequential flow for S3 configuration
func CreateS3Flow() *FlowModel {
	steps := []Step{
		CreateS3BucketStep("bucket", "my-static-site"),
		CreateAWSRegionStep("region"),
		CreateAWSAccessKeyStep("access_key"),
		CreateAWSSecretKeyStep("secret_key"),
	}

	return NewFlow("Configure S3 Deployment", steps)
}

// RunDigitalOceanFlow runs the DigitalOcean configuration flow
func RunDigitalOceanFlow(projectName string) (*config.DigitalOceanConfig, error) {
	flow := CreateDigitalOceanFlow(projectName)

	p := tea.NewProgram(flow, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("configuration not completed")
	}

	// Extract results
	results := final.GetResults()

	// Get SSH key information
	sshKeyPath, sshKeyName := final.GetSSHKeyInfo("ssh_key")

	return &config.DigitalOceanConfig{
		IP:         results["ip"],
		SSHKey:     sshKeyPath,
		SSHKeyName: sshKeyName,
		Username:   results["username"],
	}, nil
}

// RunS3Flow runs the S3 configuration flow
func RunS3Flow() (*config.S3Config, error) {
	flow := CreateS3Flow()

	p := tea.NewProgram(flow, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(FlowModel)
	if final.Cancelled {
		return nil, fmt.Errorf("configuration cancelled")
	}

	if !final.Completed {
		return nil, fmt.Errorf("configuration not completed")
	}

	// Extract results
	results := final.GetResults()

	return &config.S3Config{
		Bucket:    results["bucket"],
		Region:    results["region"],
		AccessKey: results["access_key"],
		SecretKey: results["secret_key"],
	}, nil
}

// GetProjectNameFromPath extracts a project name from a path for SSH key naming
func GetProjectNameFromPath(projectPath string) string {
	// Get the base directory name
	projectName := filepath.Base(projectPath)
	if projectName == "." || projectName == "/" {
		// Fall back to parent directory if current dir
		parent := filepath.Dir(projectPath)
		projectName = filepath.Base(parent)
	}
	return projectName
}