package sequential

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// StepBuilder provides a fluent interface for creating steps
type StepBuilder struct {
	step Step
}

// NewStep creates a new step builder
func NewStep(id, title string) *StepBuilder {
	return &StepBuilder{
		step: Step{
			ID:    id,
			Title: title,
			Type:  StepTypeText,
		},
	}
}

// Description sets the step description
func (b *StepBuilder) Description(desc string) *StepBuilder {
	b.step.Description = desc
	return b
}

// Type sets the step type
func (b *StepBuilder) Type(stepType StepType) *StepBuilder {
	b.step.Type = stepType
	return b
}

// Placeholder sets the placeholder text
func (b *StepBuilder) Placeholder(placeholder string) *StepBuilder {
	b.step.Placeholder = placeholder
	return b
}

// DefaultValue sets the default value
func (b *StepBuilder) DefaultValue(value string) *StepBuilder {
	b.step.Value = value
	return b
}

// Required marks the step as required
func (b *StepBuilder) Required() *StepBuilder {
	b.step.Required = true
	return b
}

// Validate sets a custom validation function
func (b *StepBuilder) Validate(fn func(string) error) *StepBuilder {
	b.step.Validate = fn
	return b
}

// Options sets available options for choice-based steps
func (b *StepBuilder) Options(options ...string) *StepBuilder {
	b.step.Options = options
	return b
}

// Build returns the constructed step
func (b *StepBuilder) Build() Step {
	return b.step
}

// Common validation functions

// ValidateIP validates an IP address
func ValidateIP(value string) error {
	if value == "" {
		return fmt.Errorf("IP address is required")
	}
	if net.ParseIP(value) == nil {
		return fmt.Errorf("invalid IP address format")
	}
	return nil
}

// ValidateRequired validates that a value is not empty
func ValidateRequired(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("this field is required")
	}
	return nil
}

// ValidateUsername validates a username format
func ValidateUsername(value string) error {
	if value == "" {
		return fmt.Errorf("username is required")
	}

	// Basic username validation (alphanumeric and common chars)
	matched, err := regexp.MatchString("^[a-zA-Z0-9_.-]+$", value)
	if err != nil || !matched {
		return fmt.Errorf("username contains invalid characters")
	}

	return nil
}

// ValidateSSHKeyPath validates an SSH key path
func ValidateSSHKeyPath(value string) error {
	if value == "" {
		return fmt.Errorf("SSH key path is required")
	}

	// Expand home directory if needed
	if strings.HasPrefix(value, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory")
		}
		value = filepath.Join(homeDir, value[2:])
	}

	// Check if file exists
	if _, err := os.Stat(value); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", value)
	}

	// Basic check for SSH key format
	content, err := os.ReadFile(value)
	if err != nil {
		return fmt.Errorf("cannot read SSH key file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "PRIVATE KEY") && !strings.Contains(contentStr, "BEGIN RSA PRIVATE KEY") {
		return fmt.Errorf("file does not appear to be a valid SSH private key")
	}

	return nil
}

// ValidateSSHKeyContent validates SSH key content
func ValidateSSHKeyContent(value string) error {
	if value == "" {
		return fmt.Errorf("SSH key content is required")
	}

	// Check for SSH key headers
	if !strings.Contains(value, "BEGIN") || !strings.Contains(value, "PRIVATE KEY") {
		return fmt.Errorf("invalid SSH key format")
	}

	return nil
}

// ValidateS3Bucket validates an S3 bucket name
func ValidateS3Bucket(value string) error {
	if value == "" {
		return fmt.Errorf("bucket name is required")
	}

	// Basic S3 bucket name validation
	if len(value) < 3 || len(value) > 63 {
		return fmt.Errorf("bucket name must be between 3 and 63 characters")
	}

	matched, err := regexp.MatchString("^[a-z0-9][a-z0-9.-]*[a-z0-9]$", value)
	if err != nil || !matched {
		return fmt.Errorf("bucket name contains invalid characters")
	}

	return nil
}

// ValidateAWSRegion validates an AWS region
func ValidateAWSRegion(value string) error {
	if value == "" {
		return fmt.Errorf("AWS region is required")
	}

	// Basic AWS region format validation
	matched, err := regexp.MatchString("^[a-z0-9-]+$", value)
	if err != nil || !matched {
		return fmt.Errorf("invalid AWS region format")
	}

	return nil
}

// Pre-built step factories

// CreateIPStep creates an IP address input step
func CreateIPStep(id, placeholder string) Step {
	return NewStep(id, "Server IP Address").
		Type(StepTypeText).
		Placeholder(placeholder).
		Required().
		Validate(ValidateIP).
		Build()
}

// CreateUsernameStep creates a username input step
func CreateUsernameStep(id string, defaultValue string) Step {
	return NewStep(id, "Username").
		Type(StepTypeText).
		DefaultValue(defaultValue).
		Placeholder("root").
		Required().
		Validate(ValidateUsername).
		Build()
}

// CreateSSHKeyStep creates an SSH key input step
func CreateSSHKeyStep(id string) Step {
	homeDir, _ := os.UserHomeDir()
	defaultPath := filepath.Join(homeDir, ".ssh", "id_rsa")

	return NewStep(id, "SSH Key").
		Type(StepTypeSSHKey).
		DefaultValue(defaultPath).
		Placeholder("Enter path or paste key content").
		Required().
		Build()
}

// CreateS3BucketStep creates an S3 bucket name input step
func CreateS3BucketStep(id, placeholder string) Step {
	return NewStep(id, "S3 Bucket Name").
		Type(StepTypeText).
		Placeholder(placeholder).
		Required().
		Validate(ValidateS3Bucket).
		Build()
}

// CreateAWSRegionStep creates an AWS region input step
func CreateAWSRegionStep(id string) Step {
	return NewStep(id, "AWS Region").
		Type(StepTypeText).
		DefaultValue("us-east-1").
		Placeholder("us-east-1").
		Required().
		Validate(ValidateAWSRegion).
		Build()
}

// CreateAWSAccessKeyStep creates an AWS access key input step
func CreateAWSAccessKeyStep(id string) Step {
	return NewStep(id, "AWS Access Key (optional)").
		Type(StepTypeText).
		Placeholder("Leave empty to use default credentials").
		Build()
}

// CreateAWSSecretKeyStep creates an AWS secret key input step
func CreateAWSSecretKeyStep(id string) Step {
	return NewStep(id, "AWS Secret Key (optional)").
		Type(StepTypePassword).
		Placeholder("Leave empty to use default credentials").
		Build()
}