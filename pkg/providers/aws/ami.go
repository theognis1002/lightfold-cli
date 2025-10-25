package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// AMI cache to avoid repeated lookups
var (
	amiCache      = make(map[string]string)
	amiCacheMutex sync.RWMutex
)

// getUbuntu2204AMI resolves the Ubuntu 22.04 LTS AMI for a specific region
// Uses SSM Parameter Store for latest AMI lookup with fallback to hardcoded map
func getUbuntu2204AMI(ctx context.Context, client *ec2.Client, region, imageSpec string) (string, error) {
	// Handle different image specifications
	var ssmParameter string

	if strings.Contains(imageSpec, "24.04") || strings.Contains(imageSpec, "ubuntu-24.04") {
		ssmParameter = "/aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id"
	} else {
		// Default to 22.04
		ssmParameter = "/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp2/ami-id"
	}

	cacheKey := fmt.Sprintf("%s:%s", region, ssmParameter)

	// Check cache first
	amiCacheMutex.RLock()
	if cachedAMI, ok := amiCache[cacheKey]; ok {
		amiCacheMutex.RUnlock()
		return cachedAMI, nil
	}
	amiCacheMutex.RUnlock()

	// Try to get AMI from SSM Parameter Store
	cfg := aws.Config{Region: region}
	ssmClient := ssm.NewFromConfig(cfg)

	output, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(ssmParameter),
	})

	if err == nil && output.Parameter != nil && output.Parameter.Value != nil {
		amiID := aws.ToString(output.Parameter.Value)

		// Cache the result
		amiCacheMutex.Lock()
		amiCache[cacheKey] = amiID
		amiCacheMutex.Unlock()

		return amiID, nil
	}

	// Fallback to hardcoded AMI map
	var amiMap map[string]string

	if strings.Contains(imageSpec, "24.04") {
		amiMap = ubuntu2404AMIMap
	} else {
		amiMap = ubuntu2204AMIMap
	}

	if amiID, ok := amiMap[region]; ok {
		// Cache the fallback result
		amiCacheMutex.Lock()
		amiCache[cacheKey] = amiID
		amiCacheMutex.Unlock()

		return amiID, nil
	}

	return "", &providers.ProviderError{
		Provider: "aws",
		Code:     "ami_not_found",
		Message:  fmt.Sprintf("Ubuntu AMI not found for region %s", region),
		Details: map[string]interface{}{
			"region":     region,
			"image_spec": imageSpec,
			"error":      "No AMI found in SSM or hardcoded map",
		},
	}
}

// ubuntu2204AMIMap contains hardcoded Ubuntu 22.04 LTS AMI IDs per region
// These are fallback values if SSM Parameter Store lookup fails
// Updated as of January 2025
var ubuntu2204AMIMap = map[string]string{
	"us-east-1":      "ami-0557a15b87f6559cf",
	"us-east-2":      "ami-00eeedc4036573771",
	"us-west-1":      "ami-0f8e81a3da6e2510a",
	"us-west-2":      "ami-0aff18ec83b712f05",
	"af-south-1":     "ami-0e1a96a93e1f11f37",
	"ap-east-1":      "ami-0e6f7a654eb9c8d27",
	"ap-south-1":     "ami-0a7cfb4b1c5a83268",
	"ap-northeast-1": "ami-0a0b7b240264a48d7",
	"ap-northeast-2": "ami-0c9c942bd7bf113a2",
	"ap-northeast-3": "ami-0c5fbee1f48e9f774",
	"ap-southeast-1": "ami-0dc2d3e4c0f9ebd18",
	"ap-southeast-2": "ami-0361bbf2b99f46c1d",
	"ca-central-1":   "ami-0a2e7efb4a4f5c67e",
	"eu-central-1":   "ami-0a485299a1553bd14",
	"eu-west-1":      "ami-0c1c30571d2dae5c9",
	"eu-west-2":      "ami-0b9932f4918a00c4f",
	"eu-west-3":      "ami-00d81861317c2cc1f",
	"eu-north-1":     "ami-0014ce3e52359afbd",
	"eu-south-1":     "ami-0c90ad93ee8c2f23e",
	"me-south-1":     "ami-0f52dc4cbb0e797c1",
	"sa-east-1":      "ami-0af6e9042ea5a4e3e",
}

// ubuntu2404AMIMap contains hardcoded Ubuntu 24.04 LTS AMI IDs per region
// Updated as of January 2025
var ubuntu2404AMIMap = map[string]string{
	"us-east-1":      "ami-0e2c8caa4b6378d8c",
	"us-east-2":      "ami-036841078a4b68e14",
	"us-west-1":      "ami-0da424eb883458071",
	"us-west-2":      "ami-05134c8ef96964280",
	"af-south-1":     "ami-0b761332115c38669",
	"ap-east-1":      "ami-0f65c8b975a8af0c3",
	"ap-south-1":     "ami-0e53db6fd757e38c7",
	"ap-northeast-1": "ami-0b20f552f63953f0e",
	"ap-northeast-2": "ami-0e9bfdb247cc8de84",
	"ap-northeast-3": "ami-0c7fb6c0db57eef61",
	"ap-southeast-1": "ami-047126e50991d067b",
	"ap-southeast-2": "ami-0375ab65ee943a2a6",
	"ca-central-1":   "ami-0ea18256de20ecdfc",
	"eu-central-1":   "ami-0084a47cc718c111a",
	"eu-west-1":      "ami-0e9085e60087ce171",
	"eu-west-2":      "ami-0b9fd8b55a6e3c9d5",
	"eu-west-3":      "ami-04a92520784b93e73",
	"eu-north-1":     "ami-08eb150f611ca277f",
	"eu-south-1":     "ami-09d23d5c5c3c1e24d",
	"me-south-1":     "ami-0a7ea5f1a5b5f1e1f",
	"sa-east-1":      "ami-0c820c196a818d66a",
}
