// Package aws provides AWS EC2 provider implementation for Lightfold.
//
// This package implements the providers.Provider interface for AWS EC2,
// enabling automatic provisioning, deployment, and management of EC2 instances.
//
// # Features
//
//   - Automatic security group creation with HTTP/HTTPS/SSH rules
//   - Support for Ubuntu 22.04 and 24.04 LTS with automatic AMI lookup
//   - Default VPC and subnet auto-discovery
//   - Elastic IP support (optional, configurable during provisioning)
//   - Comprehensive error handling with retry logic for rate limiting
//   - Resource cleanup on destroy (instances, security groups, Elastic IPs)
//
// # Authentication
//
// Supports multiple authentication methods:
//   - AWS Access Key ID + Secret Access Key (explicit credentials)
//   - AWS Profile from ~/.aws/credentials (profile-based)
//   - Environment variables (AWS_PROFILE, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
//
// # Resource Tagging
//
// All created resources are tagged with:
//   - lightfold:target={target-name} - Identifies the deployment target
//   - lightfold:managed=true - Marks resources as managed by Lightfold
//
// # Example Usage
//
//	client := NewClient(credentialsJSON)
//	server, err := client.Provision(ctx, providers.ProvisionConfig{
//	    Region: "us-east-1",
//	    Size:   "t3.small",
//	    Name:   "my-app",
//	    Image:  "ubuntu-22.04-amd64",
//	    SSHKeys: []string{"my-key"},
//	})
package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Register the AWS provider with the global registry
func init() {
	providers.Register("aws", func(token string) providers.Provider {
		return NewClient(token)
	})
}

// Client implements the providers.Provider interface for AWS EC2.
// It handles EC2 instance provisioning, management, and cleanup operations.
type Client struct {
	ec2Client *ec2.Client    // EC2 service client
	awsConfig aws.Config     // AWS SDK configuration
	creds     AWSCredentials // Parsed authentication credentials
}

// NewClient creates a new AWS provider client with the given credentials.
// The credentials can be provided as JSON containing access keys or profile name.
//
// Supported credential formats:
//   - {"access_key_id": "AKIA...", "secret_access_key": "..."}
//   - {"profile": "default"}
//   - Plain access key string (for backward compatibility)
//
// If no credentials are provided, it falls back to the AWS default credential chain.
func NewClient(credentialsJSON string) *Client {
	creds, err := parseCredentials(credentialsJSON)
	if err != nil {
		// Return a client with invalid credentials, validation will catch this
		return &Client{
			creds: AWSCredentials{},
		}
	}

	ctx := context.Background()
	var awsConfig aws.Config

	// Load config based on authentication method
	if creds.Profile != "" {
		// Use AWS profile
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithSharedConfigProfile(creds.Profile),
		)
	} else if creds.AccessKeyID != "" && creds.SecretAccessKey != "" {
		// Use explicit credentials
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				creds.AccessKeyID,
				creds.SecretAccessKey,
				"",
			)),
		)
	} else {
		// Try to use default credentials chain
		awsConfig, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		return &Client{
			creds: creds,
		}
	}

	return &Client{
		ec2Client: ec2.NewFromConfig(awsConfig),
		awsConfig: awsConfig,
		creds:     creds,
	}
}

func (c *Client) Name() string {
	return "aws"
}

func (c *Client) DisplayName() string {
	return "AWS EC2"
}

func (c *Client) SupportsProvisioning() bool {
	return true
}

func (c *Client) SupportsBYOS() bool {
	return true
}

func (c *Client) SupportsSSH() bool {
	return true
}

func (c *Client) ValidateCredentials(ctx context.Context) error {
	if c.ec2Client == nil {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "client_not_initialized",
			Message:  "AWS client not initialized - check credentials",
		}
	}

	// Test credentials by listing regions
	_, err := c.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "invalid_credentials",
			Message:  "Invalid AWS credentials",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return nil
}

func (c *Client) GetRegions(ctx context.Context) ([]providers.Region, error) {
	output, err := c.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(false), // Only enabled regions
	})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "list_regions_failed",
			Message:  "Failed to list AWS regions",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	var regions []providers.Region
	for _, region := range output.Regions {
		if region.RegionName != nil {
			regions = append(regions, providers.Region{
				ID:       *region.RegionName,
				Name:     getRegionDisplayName(*region.RegionName),
				Location: getRegionDisplayName(*region.RegionName),
			})
		}
	}

	return regions, nil
}

func (c *Client) GetSizes(ctx context.Context, region string) ([]providers.Size, error) {
	// Return curated list of common instance types
	// Prices are approximate and should be verified with AWS pricing API
	sizes := []providers.Size{
		{
			ID:           "t3.micro",
			Name:         "t3.micro (1 vCPU, 1 GB RAM) - Burstable",
			Memory:       1024,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 7.50,
			PriceHourly:  0.0104,
		},
		{
			ID:           "t3.small",
			Name:         "t3.small (2 vCPUs, 2 GB RAM) - Burstable",
			Memory:       2048,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 15.00,
			PriceHourly:  0.0208,
		},
		{
			ID:           "t3.medium",
			Name:         "t3.medium (2 vCPUs, 4 GB RAM) - Burstable",
			Memory:       4096,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 30.00,
			PriceHourly:  0.0416,
		},
		{
			ID:           "t3.large",
			Name:         "t3.large (2 vCPUs, 8 GB RAM) - Burstable",
			Memory:       8192,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 60.00,
			PriceHourly:  0.0832,
		},
		{
			ID:           "m5.large",
			Name:         "m5.large (2 vCPUs, 8 GB RAM) - General Purpose",
			Memory:       8192,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 70.00,
			PriceHourly:  0.096,
		},
		{
			ID:           "m5.xlarge",
			Name:         "m5.xlarge (4 vCPUs, 16 GB RAM) - General Purpose",
			Memory:       16384,
			VCPUs:        4,
			Disk:         8,
			PriceMonthly: 140.00,
			PriceHourly:  0.192,
		},
		{
			ID:           "c5.large",
			Name:         "c5.large (2 vCPUs, 4 GB RAM) - Compute Optimized",
			Memory:       4096,
			VCPUs:        2,
			Disk:         8,
			PriceMonthly: 62.00,
			PriceHourly:  0.085,
		},
	}

	return sizes, nil
}

func (c *Client) GetImages(ctx context.Context) ([]providers.Image, error) {
	// Return default Ubuntu 22.04 image
	// Actual AMI ID will be resolved per-region during provisioning
	return []providers.Image{
		{
			ID:           providers.GetDefaultImage("aws"),
			Name:         "Ubuntu 22.04 LTS",
			Distribution: "Ubuntu",
			Version:      "22.04",
		},
		{
			ID:           "ubuntu-24.04",
			Name:         "Ubuntu 24.04 LTS",
			Distribution: "Ubuntu",
			Version:      "24.04",
		},
	}, nil
}

func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
	input := &ec2.ImportKeyPairInput{
		KeyName:           aws.String(name),
		PublicKeyMaterial: []byte(publicKey),
	}

	output, err := c.ec2Client.ImportKeyPair(ctx, input)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "upload_ssh_key_failed",
			Message:  "Failed to upload SSH key to AWS",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return &providers.SSHKey{
		ID:          *output.KeyName,
		Name:        *output.KeyName,
		Fingerprint: aws.ToString(output.KeyFingerprint),
		PublicKey:   publicKey,
	}, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
	// Validate inputs
	if err := validateProvisionConfig(config); err != nil {
		return nil, err
	}

	// Get region-specific client
	regionClient, err := c.getRegionClient(config.Region)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "region_client_failed",
			Message:  fmt.Sprintf("Failed to create client for region %s", config.Region),
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	// Resolve AMI for the region
	amiID, err := getUbuntu2204AMI(ctx, regionClient, config.Region, config.Image)
	if err != nil {
		return nil, err
	}

	// Find default VPC and subnet
	vpcID, subnetID, err := findDefaultVPCAndSubnet(ctx, regionClient, config.Region)
	if err != nil {
		return nil, err
	}

	// Create security group
	sgID, err := createSecurityGroup(ctx, regionClient, vpcID, config.Name)
	if err != nil {
		return nil, err
	}

	// Prepare user data (cloud-init)
	var userData *string
	if config.UserData != "" {
		userData = aws.String(config.UserData)
	}

	// Build tag specifications
	tagSpecs := []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("lightfold-%s", config.Name))},
				{Key: aws.String("lightfold:target"), Value: aws.String(config.Name)},
				{Key: aws.String("lightfold:managed"), Value: aws.String("true")},
			},
		},
	}

	// Add custom tags
	if len(config.Tags) > 0 {
		for _, tag := range config.Tags {
			tagSpecs[0].Tags = append(tagSpecs[0].Tags, types.Tag{
				Key:   aws.String(tag),
				Value: aws.String("true"),
			})
		}
	}

	// Create EC2 instance with retry logic for rate limiting
	var runOutput *ec2.RunInstancesOutput
	runInput := &ec2.RunInstancesInput{
		ImageId:           aws.String(amiID),
		InstanceType:      types.InstanceType(config.Size),
		MinCount:          aws.Int32(1),
		MaxCount:          aws.Int32(1),
		KeyName:           aws.String(config.SSHKeys[0]), // First SSH key
		SecurityGroupIds:  []string{sgID},
		SubnetId:          aws.String(subnetID),
		UserData:          userData,
		TagSpecifications: tagSpecs,
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					VolumeSize:          aws.Int32(20), // 20GB root volume
					VolumeType:          types.VolumeTypeGp3,
					DeleteOnTermination: aws.Bool(true),
				},
			},
		},
	}

	err = retryWithBackoff(ctx, "RunInstances", func() error {
		var runErr error
		runOutput, runErr = regionClient.RunInstances(ctx, runInput)
		return runErr
	})
	if err != nil {
		return nil, enhanceError(err, "create instance")
	}

	if len(runOutput.Instances) == 0 {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "no_instance_created",
			Message:  "No instance was created",
		}
	}

	instance := &runOutput.Instances[0]

	// Store security group ID in instance metadata for cleanup
	server := convertInstanceToServer(instance)
	if server.Metadata == nil {
		server.Metadata = make(map[string]string)
	}
	server.Metadata["security_group_id"] = sgID
	server.Metadata["vpc_id"] = vpcID
	server.Metadata["subnet_id"] = subnetID

	return server, nil
}

// GetServer retrieves details about an EC2 instance by its ID.
// Returns a providers.Server struct with instance information including:
//   - Public and private IP addresses
//   - Instance state (running, stopped, terminated, etc.)
//   - Instance type, AMI, region, tags
//   - Metadata (VPC, subnet, security groups, etc.)
func (c *Client) GetServer(ctx context.Context, serverID string) (*providers.Server, error) {
	// Extract region from instance ID if needed (instances are region-specific)
	// For now, we'll use the configured region
	output, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{serverID},
	})
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "get_instance_failed",
			Message:  "Failed to get EC2 instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "instance_not_found",
			Message:  fmt.Sprintf("Instance not found: %s", serverID),
		}
	}

	return convertInstanceToServer(&output.Reservations[0].Instances[0]), nil
}

// Destroy terminates an EC2 instance and cleans up associated resources.
// This includes:
//   - Terminating the EC2 instance
//   - Waiting for termination to complete
//   - Releasing Elastic IP (if allocated by Lightfold)
//   - Deleting security group (if created by Lightfold)
//
// Resource cleanup is performed with retry logic to handle dependency violations.
// Cleanup failures are logged as warnings but don't fail the operation.
func (c *Client) Destroy(ctx context.Context, serverID string) error {
	// Get instance details to extract security group
	instance, err := c.GetServer(ctx, serverID)
	if err != nil {
		return err
	}

	// Terminate instance
	_, err = c.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{serverID},
	})
	if err != nil {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "terminate_instance_failed",
			Message:  "Failed to terminate EC2 instance",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	// Wait for instance to terminate before cleaning up resources
	waiter := ec2.NewInstanceTerminatedWaiter(c.ec2Client)
	if err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{serverID},
	}, 5*time.Minute); err != nil {
		// Log warning but don't fail - instance may already be terminated
		fmt.Printf("Warning: Failed to wait for instance termination: %v\n", err)
	}

	// Clean up Elastic IP if present
	if eipAllocationID, ok := instance.Metadata["elastic_ip_allocation_id"]; ok && eipAllocationID != "" {
		if err := releaseElasticIP(ctx, c.ec2Client, eipAllocationID); err != nil {
			fmt.Printf("Warning: Failed to release Elastic IP: %v\n", err)
		}
	}

	// Clean up security group if created by Lightfold
	if sgID, ok := instance.Metadata["security_group_id"]; ok && sgID != "" {
		if err := deleteSecurityGroup(ctx, c.ec2Client, sgID); err != nil {
			fmt.Printf("Warning: Failed to delete security group: %v\n", err)
		}
	}

	return nil
}

// WaitForActive waits for an EC2 instance to reach the "running" state.
// Uses AWS SDK's built-in waiter with exponential backoff polling.
//
// The timeout parameter specifies the maximum time to wait.
// Typical instance startup time: 30-90 seconds for t3 instances.
func (c *Client) WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*providers.Server, error) {
	waiter := ec2.NewInstanceRunningWaiter(c.ec2Client)

	err := waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{serverID},
	}, timeout)
	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "timeout",
			Message:  fmt.Sprintf("Timeout waiting for instance to become active (waited %s)", timeout.String()),
			Details:  map[string]interface{}{"timeout": timeout.String(), "error": err.Error()},
		}
	}

	return c.GetServer(ctx, serverID)
}

// getRegionClient returns an EC2 client configured for a specific region
func (c *Client) getRegionClient(region string) (*ec2.Client, error) {
	// Create a new config with the specified region
	cfg := c.awsConfig.Copy()
	cfg.Region = region
	return ec2.NewFromConfig(cfg), nil
}

// getRegionDisplayName returns a human-readable name for AWS regions
func getRegionDisplayName(regionID string) string {
	regionNames := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"us-east-2":      "US East (Ohio)",
		"us-west-1":      "US West (N. California)",
		"us-west-2":      "US West (Oregon)",
		"af-south-1":     "Africa (Cape Town)",
		"ap-east-1":      "Asia Pacific (Hong Kong)",
		"ap-south-1":     "Asia Pacific (Mumbai)",
		"ap-northeast-1": "Asia Pacific (Tokyo)",
		"ap-northeast-2": "Asia Pacific (Seoul)",
		"ap-northeast-3": "Asia Pacific (Osaka)",
		"ap-southeast-1": "Asia Pacific (Singapore)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"ca-central-1":   "Canada (Central)",
		"eu-central-1":   "Europe (Frankfurt)",
		"eu-west-1":      "Europe (Ireland)",
		"eu-west-2":      "Europe (London)",
		"eu-west-3":      "Europe (Paris)",
		"eu-north-1":     "Europe (Stockholm)",
		"eu-south-1":     "Europe (Milan)",
		"me-south-1":     "Middle East (Bahrain)",
		"sa-east-1":      "South America (São Paulo)",
	}

	if name, ok := regionNames[regionID]; ok {
		return name
	}
	return regionID
}
