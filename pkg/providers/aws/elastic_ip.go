package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// allocateElasticIP allocates a new Elastic IP address
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

// releaseElasticIP releases an Elastic IP address
// Note: EC2 instance termination automatically disassociates EIPs,
// so explicit disassociation is not needed before release.
func releaseElasticIP(ctx context.Context, client *ec2.Client, allocationID string) error {
	if allocationID == "" {
		return nil
	}

	_, err := client.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: aws.String(allocationID),
	})

	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "InvalidAllocationID.NotFound") {
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
