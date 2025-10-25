package aws

import (
	"fmt"
	"lightfold/pkg/providers"
	"strings"
)

// validateProvisionConfig validates the provision configuration before creating resources
func validateProvisionConfig(config providers.ProvisionConfig) error {
	// Validate region
	if config.Region == "" {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_config",
			Message:  "Region is required",
			Details: map[string]interface{}{
				"field": "region",
			},
		}
	}

	// Validate instance size/type
	if config.Size == "" {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_config",
			Message:  "Instance type is required",
			Details: map[string]interface{}{
				"field": "size",
			},
		}
	}

	// Validate instance type format (should match AWS format like t3.micro)
	if !isValidInstanceType(config.Size) {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_instance_type",
			Message:  fmt.Sprintf("Invalid instance type: %s", config.Size),
			Details: map[string]interface{}{
				"size":       config.Size,
				"expected":   "format like t3.micro, t3.small, m5.large, etc.",
				"next_steps": []string{"Choose from: t3.micro, t3.small, t3.medium, t3.large, m5.large, m5.xlarge, c5.large"},
			},
		}
	}

	// Validate target name
	if config.Name == "" {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_config",
			Message:  "Target name is required",
			Details: map[string]interface{}{
				"field": "name",
			},
		}
	}

	// Validate target name format (no spaces, special chars)
	if !isValidTargetName(config.Name) {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_target_name",
			Message:  "Target name must contain only alphanumeric characters, hyphens, and underscores",
			Details: map[string]interface{}{
				"name":       config.Name,
				"next_steps": []string{"Use only letters, numbers, hyphens (-), and underscores (_)", "No spaces or special characters"},
			},
		}
	}

	// Validate SSH keys
	if len(config.SSHKeys) == 0 {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_config",
			Message:  "At least one SSH key is required",
			Details: map[string]interface{}{
				"field": "ssh_keys",
			},
		}
	}

	// Validate image specification
	if config.Image == "" {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_config",
			Message:  "Image specification is required",
			Details: map[string]interface{}{
				"field": "image",
			},
		}
	}

	return nil
}

// isValidInstanceType checks if the instance type follows AWS naming conventions
func isValidInstanceType(instanceType string) bool {
	// AWS instance types follow pattern: <family>.<size>
	// Examples: t3.micro, m5.large, c5.xlarge, r6i.2xlarge
	parts := strings.Split(instanceType, ".")
	if len(parts) != 2 {
		return false
	}

	family := parts[0]
	size := parts[1]

	// Family should be 2-4 characters (t3, m5, c5, r6i, etc.)
	if len(family) < 2 || len(family) > 4 {
		return false
	}

	// Size should be non-empty
	if size == "" {
		return false
	}

	// Valid sizes include: micro, small, medium, large, xlarge, 2xlarge, etc.
	validSizes := []string{"nano", "micro", "small", "medium", "large", "xlarge", "2xlarge", "4xlarge", "8xlarge", "12xlarge", "16xlarge", "24xlarge", "32xlarge", "48xlarge", "metal"}
	for _, validSize := range validSizes {
		if size == validSize {
			return true
		}
	}

	return false
}

// isValidTargetName checks if the target name is valid
func isValidTargetName(name string) bool {
	if name == "" {
		return false
	}

	// Check each character
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_') {
			return false
		}
	}

	return true
}

// validateRegion checks if a region string is valid
// Reserved for additional region validation if needed
//
//nolint:unused // Helper function for potential future use
func validateRegion(region string) error {
	if region == "" {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_region",
			Message:  "Region cannot be empty",
		}
	}

	// AWS regions follow pattern: <location>-<direction>-<number>
	// Examples: us-east-1, eu-west-2, ap-southeast-1
	parts := strings.Split(region, "-")
	if len(parts) < 3 {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_region",
			Message:  fmt.Sprintf("Invalid AWS region format: %s", region),
			Details: map[string]interface{}{
				"region":   region,
				"expected": "format like us-east-1, eu-west-2, ap-southeast-1",
			},
		}
	}

	return nil
}
