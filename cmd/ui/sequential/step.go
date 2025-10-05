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

// Hetzner Cloud Steps

func CreateHetznerAPITokenStep(id string) Step {
	return NewStep(id, "Hetzner Cloud API Token").
		Type(StepTypePassword).
		Placeholder("hcloud_...").
		Required().
		Validate(ValidateRequired).
		Build()
}

func CreateHetznerLocationStep(id string) Step {
	// Official Hetzner Cloud locations as of 2025 (verified from docs.hetzner.com)
	locations := []string{"fsn1", "nbg1", "hel1", "ash", "hil", "sin"}
	locationDescs := []string{
		"Falkenstein, Germany",
		"Nuremberg, Germany",
		"Helsinki, Finland",
		"Ashburn, VA, USA",
		"Hillsboro, OR, USA",
		"Singapore",
	}

	return NewStep(id, "Hetzner Location").
		Type(StepTypeSelect).
		DefaultValue("fsn1").
		Options(locations...).
		OptionDescriptions(locationDescs...).
		Required().
		Build()
}

func CreateHetznerServerTypeStep(id string) Step {
	// Current Hetzner server types (2025):
	// - CPX series (AMD EPYC): cpx11, cpx21, cpx31, cpx41, cpx51
	// - New CX series (Intel): cx22, cx32, cx42, cx52
	// Old CX series (cx11-cx51) deprecated Sept 2024 but may still appear in API
	serverTypes := []string{"cpx11", "cpx21", "cpx31", "cpx41", "cx22", "cx32", "cx42", "cx52"}
	serverTypeDescs := []string{
		"2 vCPU, 2 GB RAM, 40 GB SSD (AMD)",
		"3 vCPUs, 4 GB RAM, 80 GB SSD (AMD)",
		"4 vCPUs, 8 GB RAM, 160 GB SSD (AMD)",
		"8 vCPUs, 16 GB RAM, 240 GB SSD (AMD)",
		"2 vCPUs, 4 GB RAM, 40 GB SSD (Intel)",
		"4 vCPUs, 8 GB RAM, 80 GB SSD (Intel)",
		"8 vCPUs, 16 GB RAM, 160 GB SSD (Intel)",
		"16 vCPUs, 32 GB RAM, 320 GB SSD (Intel)",
	}

	return NewStep(id, "Server Type").
		Type(StepTypeSelect).
		DefaultValue("cpx11").
		Options(serverTypes...).
		OptionDescriptions(serverTypeDescs...).
		Required().
		Build()
}

// CreateHetznerLocationStepDynamic creates a Hetzner location step with dynamic API data
func CreateHetznerLocationStepDynamic(id, token string) Step {
	ctx := context.Background()
	provider, err := providers.GetProvider("hetzner", token)
	if err != nil {
		return CreateHetznerLocationStep(id) // Fallback to static
	}

	regions, err := provider.GetRegions(ctx)
	if err != nil {
		return CreateHetznerLocationStep(id) // Fallback to static
	}

	if len(regions) == 0 {
		return CreateHetznerLocationStep(id) // Fallback to static
	}

	var locationIDs []string
	var locationDescs []string
	for _, region := range regions {
		locationIDs = append(locationIDs, region.ID)
		locationDescs = append(locationDescs, region.Location)
	}

	return NewStep(id, "Hetzner Location").
		Type(StepTypeSelect).
		DefaultValue(locationIDs[0]).
		Options(locationIDs...).
		OptionDescriptions(locationDescs...).
		Required().
		Build()
}

// CreateHetznerServerTypeStepDynamic creates a Hetzner server type step with dynamic API data
func CreateHetznerServerTypeStepDynamic(id, token, location string) Step {
	ctx := context.Background()
	provider, err := providers.GetProvider("hetzner", token)
	if err != nil {
		return CreateHetznerServerTypeStep(id) // Fallback to static
	}

	sizes, err := provider.GetSizes(ctx, location)
	if err != nil {
		return CreateHetznerServerTypeStep(id) // Fallback to static
	}

	if len(sizes) == 0 {
		return CreateHetznerServerTypeStep(id) // Fallback to static
	}

	var sizeIDs []string
	var sizeDescs []string
	for _, size := range sizes {
		sizeIDs = append(sizeIDs, size.ID)
		// Format: "cx11 - 1 vCPU, 2 GB RAM, 20 GB SSD"
		desc := fmt.Sprintf("%d vCPU, %d GB RAM, %d GB SSD", size.VCPUs, size.Memory/1024, size.Disk)
		if size.PriceMonthly > 0 {
			desc = fmt.Sprintf("%s (€%.2f/mo)", desc, size.PriceMonthly)
		}
		sizeDescs = append(sizeDescs, desc)
	}

	defaultValue := "cx11"
	if len(sizeIDs) > 0 {
		defaultValue = sizeIDs[0]
	}

	return NewStep(id, "Server Type").
		Type(StepTypeSelect).
		DefaultValue(defaultValue).
		Options(sizeIDs...).
		OptionDescriptions(sizeDescs...).
		Required().
		Build()
}

func CreateDynamicHetznerLocationStep(id string, provider providers.Provider) Step {
	ctx := context.Background()

	locations, err := provider.GetRegions(ctx)
	if err != nil {
		return CreateHetznerLocationStep(id)
	}

	var locationIDs []string
	var locationDescs []string
	for _, location := range locations {
		locationIDs = append(locationIDs, location.ID)
		locationDescs = append(locationDescs, location.Location)
	}

	defaultValue := "nbg1"
	if len(locationIDs) > 0 {
		defaultValue = locationIDs[0]
	}

	return NewStep(id, "Hetzner Location").
		Type(StepTypeSelect).
		DefaultValue(defaultValue).
		Options(locationIDs...).
		OptionDescriptions(locationDescs...).
		Required().
		Build()
}

func CreateDynamicHetznerServerTypeStep(id string, provider providers.Provider) Step {
	ctx := context.Background()

	serverTypes, err := provider.GetSizes(ctx, "")
	if err != nil {
		return CreateHetznerServerTypeStep(id)
	}

	sort.Slice(serverTypes, func(i, j int) bool {
		return serverTypes[i].Memory < serverTypes[j].Memory
	})

	var typeIDs []string
	var typeDescs []string
	for _, st := range serverTypes {
		typeIDs = append(typeIDs, st.ID)
		typeDescs = append(typeDescs, st.Name)
	}

	defaultValue := "cx11"
	if len(typeIDs) > 0 {
		defaultValue = typeIDs[0]
	}

	return NewStep(id, "Server Type").
		Type(StepTypeSelect).
		DefaultValue(defaultValue).
		Options(typeIDs...).
		OptionDescriptions(typeDescs...).
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
