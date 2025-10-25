package aws

import (
	"context"
	"fmt"
	"lightfold/pkg/providers"
	"strings"
	"time"
)

// retryConfig defines retry behavior for AWS operations
type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// defaultRetryConfig returns the default retry configuration
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxRetries: 5,
		baseDelay:  1 * time.Second,
		maxDelay:   30 * time.Second,
	}
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Rate limiting errors
	if strings.Contains(errStr, "RequestLimitExceeded") ||
		strings.Contains(errStr, "Throttling") ||
		strings.Contains(errStr, "TooManyRequests") ||
		strings.Contains(errStr, "SlowDown") {
		return true
	}

	// Network errors
	if strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "network") {
		return true
	}

	// Temporary AWS service errors
	if strings.Contains(errStr, "ServiceUnavailable") ||
		strings.Contains(errStr, "InternalError") ||
		strings.Contains(errStr, "InternalFailure") {
		return true
	}

	return false
}

// isQuotaError checks if an error is due to quota/limit issues
func isQuotaError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "InstanceLimitExceeded") ||
		strings.Contains(errStr, "VcpuLimitExceeded") ||
		strings.Contains(errStr, "AddressLimitExceeded") ||
		strings.Contains(errStr, "SecurityGroupLimitExceeded")
}

// enhanceError adds helpful context and next steps to AWS errors
func enhanceError(err error, operation string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Handle quota errors with actionable guidance
	if isQuotaError(err) {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "quota_exceeded",
			Message:  fmt.Sprintf("AWS quota limit exceeded for %s", operation),
			Details: map[string]interface{}{
				"error": errStr,
				"next_steps": []string{
					"Check your AWS quota limits in the Service Quotas console",
					"Request a quota increase for EC2 in your region",
					"Try a different region or instance type",
					"Clean up unused resources to free capacity",
				},
			},
		}
	}

	// Handle rate limiting
	if strings.Contains(errStr, "RequestLimitExceeded") ||
		strings.Contains(errStr, "Throttling") {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "rate_limited",
			Message:  fmt.Sprintf("AWS API rate limit exceeded for %s", operation),
			Details: map[string]interface{}{
				"error": errStr,
				"next_steps": []string{
					"Wait a few minutes and try again",
					"The operation will be retried automatically",
					"Consider reducing concurrent operations",
				},
			},
		}
	}

	// Handle credential errors
	if strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "AuthFailure") ||
		strings.Contains(errStr, "InvalidClientTokenId") {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "auth_failed",
			Message:  fmt.Sprintf("AWS authentication failed for %s", operation),
			Details: map[string]interface{}{
				"error": errStr,
				"next_steps": []string{
					"Verify your AWS Access Key ID and Secret Access Key",
					"Check that your credentials haven't expired",
					"Ensure your IAM user/role has required permissions",
					"Run: lightfold config set-token aws",
				},
			},
		}
	}

	// Handle permission errors
	if strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "AccessDenied") {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "permission_denied",
			Message:  fmt.Sprintf("Insufficient AWS permissions for %s", operation),
			Details: map[string]interface{}{
				"error": errStr,
				"next_steps": []string{
					"Check IAM permissions for your AWS user/role",
					"Required permissions: ec2:RunInstances, ec2:DescribeInstances, etc.",
					"See docs/AWS_SETUP.md for complete IAM policy",
					"Contact your AWS administrator if you don't have access",
				},
			},
		}
	}

	// Handle default VPC missing
	if strings.Contains(errStr, "default VPC") {
		return &providers.ProviderError{
			Provider: "aws",
			Code:     "no_default_vpc",
			Message:  "No default VPC found in the selected region",
			Details: map[string]interface{}{
				"error": errStr,
				"next_steps": []string{
					"Create a default VPC in your AWS console",
					"Run: aws ec2 create-default-vpc",
					"Or select a different region",
					"See: https://docs.aws.amazon.com/vpc/latest/userguide/default-vpc.html",
				},
			},
		}
	}

	return err
}

// retryWithBackoff executes a function with exponential backoff retry logic
func retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	cfg := defaultRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= cfg.maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !isRetryableError(err) {
			return enhanceError(err, operation)
		}

		// Don't retry if we've exhausted attempts
		if attempt == cfg.maxRetries {
			break
		}

		// Calculate backoff delay with exponential increase
		delay := cfg.baseDelay * time.Duration(1<<uint(attempt))
		if delay > cfg.maxDelay {
			delay = cfg.maxDelay
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	// All retries exhausted
	return &providers.ProviderError{
		Provider: "aws",
		Code:     "retry_exhausted",
		Message:  fmt.Sprintf("Operation %s failed after %d retries", operation, cfg.maxRetries),
		Details: map[string]interface{}{
			"error":       lastErr.Error(),
			"max_retries": cfg.maxRetries,
		},
	}
}
