package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// convertInstanceToServer converts an EC2 instance to a providers.Server
func convertInstanceToServer(instance *types.Instance) *providers.Server {
	var publicIPv4 string
	var privateIPv4 string

	if instance.PublicIpAddress != nil {
		publicIPv4 = *instance.PublicIpAddress
	}

	if instance.PrivateIpAddress != nil {
		privateIPv4 = *instance.PrivateIpAddress
	}

	// Extract instance name from tags
	instanceName := aws.ToString(instance.InstanceId)
	for _, tag := range instance.Tags {
		if aws.ToString(tag.Key) == "Name" {
			instanceName = aws.ToString(tag.Value)
			break
		}
	}

	// Build metadata
	metadata := map[string]string{
		"instance_type":  string(instance.InstanceType),
		"architecture":   string(instance.Architecture),
		"virtualization": string(instance.VirtualizationType),
		"hypervisor":     string(instance.Hypervisor),
		"placement_zone": aws.ToString(instance.Placement.AvailabilityZone),
	}

	if instance.ImageId != nil {
		metadata["image_id"] = *instance.ImageId
	}

	if instance.KeyName != nil {
		metadata["key_name"] = *instance.KeyName
	}

	if instance.VpcId != nil {
		metadata["vpc_id"] = *instance.VpcId
	}

	if instance.SubnetId != nil {
		metadata["subnet_id"] = *instance.SubnetId
	}

	// Extract security group IDs
	if len(instance.SecurityGroups) > 0 {
		metadata["security_group_id"] = aws.ToString(instance.SecurityGroups[0].GroupId)
	}

	// Extract tags
	var tagsList []string
	for _, tag := range instance.Tags {
		if aws.ToString(tag.Key) != "Name" {
			tagsList = append(tagsList, fmt.Sprintf("%s=%s", aws.ToString(tag.Key), aws.ToString(tag.Value)))
		}
	}

	var createdAt time.Time
	if instance.LaunchTime != nil {
		createdAt = *instance.LaunchTime
	}

	return &providers.Server{
		ID:          aws.ToString(instance.InstanceId),
		Name:        instanceName,
		Status:      string(instance.State.Name),
		PublicIPv4:  publicIPv4,
		PrivateIPv4: privateIPv4,
		Region:      aws.ToString(instance.Placement.AvailabilityZone),
		Size:        string(instance.InstanceType),
		Image:       aws.ToString(instance.ImageId),
		Tags:        tagsList,
		CreatedAt:   createdAt,
		Metadata:    metadata,
	}
}

// getInstanceState extracts the instance state as a string
// Note: Currently unused as convertInstanceToServer handles this directly
//
//nolint:unused // Helper function for potential future use
func getInstanceState(instance *types.Instance) string {
	if instance.State != nil {
		return string(instance.State.Name)
	}
	return "unknown"
}

// getPublicIP extracts the public IPv4 address from an instance
// Note: Currently unused as convertInstanceToServer handles this directly
//
//nolint:unused // Helper function for potential future use
func getPublicIP(instance *types.Instance) string {
	return aws.ToString(instance.PublicIpAddress)
}

// getPrivateIP extracts the private IPv4 address from an instance
// Note: Currently unused as convertInstanceToServer handles this directly
//
//nolint:unused // Helper function for potential future use
func getPrivateIP(instance *types.Instance) string {
	return aws.ToString(instance.PrivateIpAddress)
}

// findDefaultVPCAndSubnet finds the default VPC and a subnet for the given region
func findDefaultVPCAndSubnet(ctx context.Context, client *ec2.Client, region string) (vpcID string, subnetID string, err error) {
	// Find default VPC
	vpcOutput, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("is-default"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return "", "", &providers.ProviderError{
			Provider: "aws",
			Code:     "describe_vpcs_failed",
			Message:  "Failed to describe VPCs",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	if len(vpcOutput.Vpcs) == 0 {
		return "", "", &providers.ProviderError{
			Provider: "aws",
			Code:     "no_default_vpc",
			Message:  fmt.Sprintf("No default VPC found in region %s. Please create a default VPC first.", region),
		}
	}

	vpcID = aws.ToString(vpcOutput.Vpcs[0].VpcId)

	// Find a subnet in the default VPC
	subnetOutput, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("default-for-az"),
				Values: []string{"true"},
			},
		},
	})
	if err != nil {
		return "", "", &providers.ProviderError{
			Provider: "aws",
			Code:     "describe_subnets_failed",
			Message:  "Failed to describe subnets",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	if len(subnetOutput.Subnets) == 0 {
		return "", "", &providers.ProviderError{
			Provider: "aws",
			Code:     "no_default_subnet",
			Message:  fmt.Sprintf("No default subnet found in VPC %s", vpcID),
		}
	}

	subnetID = aws.ToString(subnetOutput.Subnets[0].SubnetId)

	return vpcID, subnetID, nil
}
