package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// allocateElasticIP allocates a new Elastic IP address
// TODO: Integrate into provision flow when AWS UI flow is implemented
//
//nolint:unused // Reserved for Elastic IP feature
func allocateElasticIP(ctx context.Context, client *ec2.Client, targetName string) (string, error) {
	output, err := client.AllocateAddress(ctx, &ec2.AllocateAddressInput{
		Domain: types.DomainTypeVpc,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String(fmt.Sprintf("lightfold-%s", targetName))},
					{Key: aws.String("lightfold:target"), Value: aws.String(targetName)},
					{Key: aws.String("lightfold:managed"), Value: aws.String("true")},
				},
			},
		},
	})

	if err != nil {
		return "", &providers.ProviderError{
			Provider: "aws",
			Code:     "allocate_eip_failed",
			Message:  "Failed to allocate Elastic IP",
			Details:  map[string]interface{}{"error": err.Error()},
		}
	}

	return aws.ToString(output.AllocationId), nil
}

// associateElasticIP associates an Elastic IP with an EC2 instance
// TODO: Integrate into provision flow when AWS UI flow is implemented
//
//nolint:unused // Reserved for Elastic IP feature
func associateElasticIP(ctx context.Context, client *ec2.Client, instanceID, allocationID string) (string, error) {
	output, err := client.AssociateAddress(ctx, &ec2.AssociateAddressInput{
		InstanceId:   aws.String(instanceID),
		AllocationId: aws.String(allocationID),
	})

	if err != nil {
		return "", &providers.ProviderError{
			Provider: "aws",
			Code:     "associate_eip_failed",
			Message:  "Failed to associate Elastic IP with instance",
			Details: map[string]interface{}{
				"error":         err.Error(),
				"instance_id":   instanceID,
				"allocation_id": allocationID,
			},
		}
	}

	return aws.ToString(output.AssociationId), nil
}

// disassociateElasticIP disassociates an Elastic IP from an instance
// Used in destroy flow for cleanup
//
//nolint:unused // Used in destroy flow, false positive
func disassociateElasticIP(ctx context.Context, client *ec2.Client, associationID string) error {
	if associationID == "" {
		// No association to remove
		return nil
	}

	_, err := client.DisassociateAddress(ctx, &ec2.DisassociateAddressInput{
		AssociationId: aws.String(associationID),
	})

	if err != nil {
		// Check if already disassociated
		errStr := err.Error()
		if contains(errStr, "InvalidAssociationID.NotFound") {
			// Already disassociated, not an error
			return nil
		}

		return &providers.ProviderError{
			Provider: "aws",
			Code:     "disassociate_eip_failed",
			Message:  "Failed to disassociate Elastic IP",
			Details: map[string]interface{}{
				"error":          err.Error(),
				"association_id": associationID,
			},
		}
	}

	return nil
}

// releaseElasticIP releases an Elastic IP address
func releaseElasticIP(ctx context.Context, client *ec2.Client, allocationID string) error {
	if allocationID == "" {
		// No allocation to release
		return nil
	}

	_, err := client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: aws.String(allocationID),
	})

	if err != nil {
		// Check if already released
		errStr := err.Error()
		if contains(errStr, "InvalidAllocationID.NotFound") {
			// Already released, not an error
			return nil
		}

		return &providers.ProviderError{
			Provider: "aws",
			Code:     "release_eip_failed",
			Message:  "Failed to release Elastic IP",
			Details: map[string]interface{}{
				"error":         err.Error(),
				"allocation_id": allocationID,
			},
		}
	}

	return nil
}

// getElasticIPByAllocationID retrieves Elastic IP details by allocation ID
// TODO: May be used for status/info commands
//
//nolint:unused // Reserved for future EIP status queries
func getElasticIPByAllocationID(ctx context.Context, client *ec2.Client, allocationID string) (*types.Address, error) {
	output, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		AllocationIds: []string{allocationID},
	})

	if err != nil {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "describe_eip_failed",
			Message:  "Failed to describe Elastic IP",
			Details: map[string]interface{}{
				"error":         err.Error(),
				"allocation_id": allocationID,
			},
		}
	}

	if len(output.Addresses) == 0 {
		return nil, &providers.ProviderError{
			Provider: "aws",
			Code:     "eip_not_found",
			Message:  fmt.Sprintf("Elastic IP not found: %s", allocationID),
			Details: map[string]interface{}{
				"allocation_id": allocationID,
			},
		}
	}

	return &output.Addresses[0], nil
}
