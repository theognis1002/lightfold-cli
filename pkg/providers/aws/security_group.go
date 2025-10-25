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

// createSecurityGroup creates a security group for Lightfold with required ports
func createSecurityGroup(ctx context.Context, client *ec2.Client, vpcID, targetName string) (string, error) {
	groupName := fmt.Sprintf("lightfold-%s", targetName)
	description := fmt.Sprintf("Security group for Lightfold deployment: %s", targetName)

	// Check if security group already exists
	existingGroups, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("group-name"),
				Values: []string{groupName},
			},
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
		},
	})

	if err == nil && len(existingGroups.SecurityGroups) > 0 {
		// Security group already exists, return its ID
		return aws.ToString(existingGroups.SecurityGroups[0].GroupId), nil
	}

	// Create security group
	createOutput, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String(description),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSecurityGroup,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(groupName)},
					{Key: aws.String("lightfold:target"), Value: aws.String(targetName)},
					{Key: aws.String("lightfold:managed"), Value: aws.String("true")},
				},
			},
		},
	})

	if err != nil {
		return "", &providers.ProviderError{
			Provider: "aws",
			Code:     "create_security_group_failed",
			Message:  "Failed to create security group",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	securityGroupID := aws.ToString(createOutput.GroupId)

	// Configure ingress rules
	// SSH (22), HTTP (80), HTTPS (443), App Ports (3000-9000)
	_, err = client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(securityGroupID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("SSH access"),
					},
				},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(80),
				ToPort:     aws.Int32(80),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("HTTP access"),
					},
				},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(443),
				ToPort:     aws.Int32(443),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("HTTPS access"),
					},
				},
			},
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(3000),
				ToPort:     aws.Int32(9000),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("Application ports"),
					},
				},
			},
		},
	})

	if err != nil {
		// If ingress rules fail, delete the security group and return error
		_ = deleteSecurityGroup(ctx, client, securityGroupID)
		return "", &providers.ProviderError{
			Provider: "aws",
			Code:     "authorize_ingress_failed",
			Message:  "Failed to authorize security group ingress rules",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return securityGroupID, nil
}

// deleteSecurityGroup deletes a security group with retry logic for dependency violations
func deleteSecurityGroup(ctx context.Context, client *ec2.Client, securityGroupID string) error {
	maxRetries := 10
	retryDelay := 5 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(securityGroupID),
		})

		if err == nil {
			// Successfully deleted
			return nil
		}

		// Check if error is a dependency violation
		errStr := err.Error()
		if contains(errStr, "DependencyViolation") {
			if attempt < maxRetries-1 {
				// Wait and retry
				time.Sleep(retryDelay)
				continue
			}
		}

		// If it's not a dependency violation or we've exhausted retries, return error
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "delete_security_group_failed",
			Message:  fmt.Sprintf("Failed to delete security group after %d attempts", attempt+1),
			Details: map[string]interface{}{
				"error":             err.Error(),
				"security_group_id": securityGroupID,
			},
		}
	}

	return &providers.ProviderError{
		Provider: "aws",
		Code:     "delete_security_group_timeout",
		Message:  fmt.Sprintf("Timeout waiting to delete security group (tried %d times)", maxRetries),
		Details: map[string]interface{}{
			"security_group_id": securityGroupID,
		},
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
