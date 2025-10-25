package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func createSecurityGroup(ctx context.Context, client *ec2.Client, vpcID, targetName string) (string, error) {
	groupName := fmt.Sprintf("lightfold-%s", targetName)
	description := fmt.Sprintf("Security group for Lightfold deployment: %s", targetName)

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
		return aws.ToString(existingGroups.SecurityGroups[0].GroupId), nil
	}

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

func deleteSecurityGroup(ctx context.Context, client *ec2.Client, securityGroupID string) error {
	cfg := defaultRetryConfig()

	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		_, err := client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(securityGroupID),
		})

		if err == nil {
			return nil
		}

		errStr := err.Error()
		if strings.Contains(errStr, "DependencyViolation") {
			if attempt < cfg.maxRetries {
				delay := cfg.baseDelay * time.Duration(1<<uint(attempt))
				if delay > cfg.maxDelay {
					delay = cfg.maxDelay
				}
				time.Sleep(delay)
				continue
			}
		}

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
		Message:  fmt.Sprintf("Timeout waiting to delete security group (tried %d times)", cfg.maxRetries),
		Details: map[string]interface{}{
			"security_group_id": securityGroupID,
		},
	}
}
