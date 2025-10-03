package sequential

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type StepBuilder struct {
	step Step
}

func NewStep(id, title string) *StepBuilder {
	return &StepBuilder{
		step: Step{
			ID:    id,
			Title: title,
			Type:  StepTypeText,
		},
	}
}

func (b *StepBuilder) Description(desc string) *StepBuilder {
	b.step.Description = desc
	return b
}

func (b *StepBuilder) Type(stepType StepType) *StepBuilder {
	b.step.Type = stepType
	return b
}

func (b *StepBuilder) Placeholder(placeholder string) *StepBuilder {
	b.step.Placeholder = placeholder
	return b
}

func (b *StepBuilder) DefaultValue(value string) *StepBuilder {
	b.step.Value = value
	return b
}

func (b *StepBuilder) Required() *StepBuilder {
	b.step.Required = true
	return b
}

func (b *StepBuilder) Validate(fn func(string) error) *StepBuilder {
	b.step.Validate = fn
	return b
}

func (b *StepBuilder) Options(options ...string) *StepBuilder {
	b.step.Options = options
	return b
}

func (b *StepBuilder) OptionDescriptions(descs ...string) *StepBuilder {
	b.step.OptionDescs = descs
	return b
}

func (b *StepBuilder) Build() Step {
	return b.step
}

func ValidateIP(value string) error {
	if value == "" {
		return fmt.Errorf("IP address is required")
	}
	if net.ParseIP(value) == nil {
		return fmt.Errorf("invalid IP address format")
	}
	return nil
}

func ValidateRequired(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("this field is required")
	}
	return nil
}

func ValidateUsername(value string) error {
	if value == "" {
		return fmt.Errorf("username is required")
	}

	matched, err := regexp.MatchString("^[a-zA-Z0-9_.-]+$", value)
	if err != nil || !matched {
		return fmt.Errorf("username contains invalid characters")
	}

	return nil
}

func ValidateSSHKeyPath(value string) error {
	if value == "" {
		return fmt.Errorf("SSH key path is required")
	}

	if strings.HasPrefix(value, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory")
		}
		value = filepath.Join(homeDir, value[2:])
	}

	if _, err := os.Stat(value); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", value)
	}

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

func ValidateSSHKeyContent(value string) error {
	if value == "" {
		return fmt.Errorf("SSH key content is required")
	}

	if !strings.Contains(value, "BEGIN") || !strings.Contains(value, "PRIVATE KEY") {
		return fmt.Errorf("invalid SSH key format")
	}

	return nil
}

func ValidateS3Bucket(value string) error {
	if value == "" {
		return fmt.Errorf("bucket name is required")
	}

	if len(value) < 3 || len(value) > 63 {
		return fmt.Errorf("bucket name must be between 3 and 63 characters")
	}

	matched, err := regexp.MatchString("^[a-z0-9][a-z0-9.-]*[a-z0-9]$", value)
	if err != nil || !matched {
		return fmt.Errorf("bucket name contains invalid characters")
	}

	return nil
}

func ValidateAWSRegion(value string) error {
	if value == "" {
		return fmt.Errorf("AWS region is required")
	}

	matched, err := regexp.MatchString("^[a-z0-9-]+$", value)
	if err != nil || !matched {
		return fmt.Errorf("invalid AWS region format")
	}

	return nil
}

func CreateIPStep(id, placeholder string) Step {
	return NewStep(id, "Server IP Address").
		Type(StepTypeText).
		Placeholder(placeholder).
		Required().
		Validate(ValidateIP).
		Build()
}

func CreateUsernameStep(id string, defaultValue string) Step {
	return NewStep(id, "Username").
		Type(StepTypeText).
		DefaultValue(defaultValue).
		Placeholder("root").
		Required().
		Validate(ValidateUsername).
		Build()
}

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

func CreateS3BucketStep(id, placeholder string) Step {
	return NewStep(id, "S3 Bucket Name").
		Type(StepTypeText).
		Placeholder(placeholder).
		Required().
		Validate(ValidateS3Bucket).
		Build()
}

func CreateAWSRegionStep(id string) Step {
	return NewStep(id, "AWS Region").
		Type(StepTypeText).
		DefaultValue("us-east-1").
		Placeholder("us-east-1").
		Required().
		Validate(ValidateAWSRegion).
		Build()
}

func CreateAWSAccessKeyStep(id string) Step {
	return NewStep(id, "AWS Access Key (optional)").
		Type(StepTypeText).
		Placeholder("Leave empty to use default credentials").
		Build()
}

func CreateAWSSecretKeyStep(id string) Step {
	return NewStep(id, "AWS Secret Key (optional)").
		Type(StepTypePassword).
		Placeholder("Leave empty to use default credentials").
		Build()
}

func CreateAPITokenStep(id string) Step {
	return NewStep(id, "DigitalOcean API Token").
		Type(StepTypePassword).
		Placeholder("dop_v1_...").
		Required().
		Validate(ValidateRequired).
		Build()
}

func CreateRegionStep(id string) Step {
	regions := []string{"nyc1", "nyc3", "ams3", "sfo3", "sgp1", "lon1", "fra1", "tor1", "blr1", "syd1"}
	regionDescs := []string{
		"New York City 1",
		"New York City 3",
		"Amsterdam 3",
		"San Francisco 3",
		"Singapore 1",
		"London 1",
		"Frankfurt 1",
		"Toronto 1",
		"Bangalore 1",
		"Sydney 1",
	}

	return NewStep(id, "DigitalOcean Region").
		Type(StepTypeSelect).
		DefaultValue("nyc1").
		Options(regions...).
		OptionDescriptions(regionDescs...).
		Required().
		Build()
}

func CreateSizeStep(id string) Step {
	sizes := []string{
		"s-1vcpu-512mb-10gb",
		"s-1vcpu-1gb",
		"s-1vcpu-2gb",
		"s-2vcpu-2gb",
		"s-2vcpu-4gb",
		"s-4vcpu-8gb",
		"s-6vcpu-16gb",
		"s-8vcpu-32gb",
	}
	sizeDescs := []string{
		"512 MB RAM, 1 vCPU, 10 GB SSD",
		"1 GB RAM, 1 vCPU, 25 GB SSD",
		"2 GB RAM, 1 vCPU, 50 GB SSD",
		"2 GB RAM, 2 vCPUs, 60 GB SSD",
		"4 GB RAM, 2 vCPUs, 80 GB SSD",
		"8 GB RAM, 4 vCPUs, 160 GB SSD",
		"16 GB RAM, 6 vCPUs, 320 GB SSD",
		"32 GB RAM, 8 vCPUs, 640 GB SSD",
	}

	return NewStep(id, "Droplet Size").
		Type(StepTypeSelect).
		DefaultValue("s-1vcpu-512mb-10gb").
		Options(sizes...).
		OptionDescriptions(sizeDescs...).
		Required().
		Build()
}

func CreateDynamicSizeStep(id string, provider providers.Provider, region string) Step {
	ctx := context.Background()

	apiSizes, err := provider.GetSizes(ctx, region)
	if err != nil {
		return CreateSizeStep(id)
	}

	sort.Slice(apiSizes, func(i, j int) bool {
		return apiSizes[i].Memory < apiSizes[j].Memory
	})

	var sizes []string
	var sizeDescs []string
	for _, size := range apiSizes {
		sizes = append(sizes, size.ID)
		sizeDescs = append(sizeDescs, size.Name)
	}

	defaultValue := ""
	if len(sizes) > 0 {
		defaultValue = sizes[0]
	}

	return NewStep(id, "Droplet Size").
		Type(StepTypeSelect).
		DefaultValue(defaultValue).
		Options(sizes...).
		OptionDescriptions(sizeDescs...).
		Required().
		Build()
}

func (m *FlowModel) UpdateStepWithDynamicSizes(stepID string, provider providers.Provider, region string) error {
	ctx := context.Background()

	stepIndex := -1
	for i, step := range m.Steps {
		if step.ID == stepID {
			stepIndex = i
			break
		}
	}

	if stepIndex == -1 {
		return fmt.Errorf("step %s not found", stepID)
	}

	apiSizes, err := provider.GetSizes(ctx, region)
	if err != nil {
		return err
	}

	sort.Slice(apiSizes, func(i, j int) bool {
		return apiSizes[i].Memory < apiSizes[j].Memory
	})

	var sizes []string
	var sizeDescs []string
	for _, size := range apiSizes {
		sizes = append(sizes, size.ID)
		sizeDescs = append(sizeDescs, size.Name)
	}

	step := m.Steps[stepIndex]
	step.Options = sizes
	step.OptionDescs = sizeDescs
	if len(sizes) > 0 {
		step.Value = sizes[0]
	}

	m.Steps[stepIndex] = step
	m.StepStates[stepIndex] = step

	return nil
}
