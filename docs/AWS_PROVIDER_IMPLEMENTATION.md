# AWS Provider Implementation Plan

This document outlines the complete implementation plan for adding AWS EC2 (dynamic server provisioning) and AWS S3 (static site hosting) providers to Lightfold CLI. The implementation follows the existing provider pattern used by DigitalOcean, Vultr, and Hetzner for consistency and maintainability.

---

## Overview

### Goals
1. **AWS EC2 Provider**: Full-featured dynamic server provisioning for web applications
2. **AWS S3 Provider**: Static site hosting (static sites only)
3. **Detection Enhancement**: Automatically identify static vs dynamic sites and recommend appropriate provider
4. **Consistent Design**: Follow existing patterns for DRY, maintainable code
5. **Comprehensive Testing**: Match or exceed test coverage of existing providers

### Design Principles
- **DRY (Don't Repeat Yourself)**: Reuse existing patterns and utilities
- **Provider Registry Pattern**: Auto-registration at init time
- **Interface-Based Design**: Implement `providers.Provider` interface
- **Static Site Validation**: S3 provider rejects non-static deployments
- **IP Recovery**: EC2 implements same recovery pattern as DO/Vultr/Hetzner

---

## Phase 1: AWS EC2 Provider (Dynamic Sites)

### 1.1 AWS EC2 Client Implementation

**File**: `pkg/providers/awsec2/client.go`

**Tasks**:
- [ ] Create `pkg/providers/awsec2/` directory
- [ ] Implement `Client` struct with AWS SDK v2 session
- [ ] Add `init()` function to register provider as `"awsec2"`
- [ ] Implement `Provider` interface methods:
  - [ ] `Name() string` → return `"awsec2"`
  - [ ] `DisplayName() string` → return `"AWS EC2"`
  - [ ] `SupportsProvisioning() bool` → return `true`
  - [ ] `SupportsBYOS() bool` → return `true`
  - [ ] `ValidateCredentials(ctx) error`
  - [ ] `GetRegions(ctx) ([]Region, error)`
  - [ ] `GetSizes(ctx, region) ([]Size, error)`
  - [ ] `GetImages(ctx) ([]Image, error)`
  - [ ] `Provision(ctx, config) (*Server, error)`
  - [ ] `GetServer(ctx, serverID) (*Server, error)`
  - [ ] `Destroy(ctx, serverID) error`
  - [ ] `WaitForActive(ctx, serverID, timeout) (*Server, error)`
  - [ ] `UploadSSHKey(ctx, name, publicKey) (*SSHKey, error)`

**Dependencies**:
```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
    "github.com/aws/aws-sdk-go-v2/credentials"
)
```

**Key Implementation Details**:
- Use AWS SDK v2 for modern API support
- Handle AWS credentials via access key + secret key (stored in tokens.json as JSON)
- Map EC2 instance states to provider status strings
- Convert EC2 API responses to `providers.Server` struct
- Implement error wrapping with `providers.ProviderError`
- Default to Ubuntu 22.04 LTS AMI when listing images
- Filter instance types to minimum 512MB RAM (like other providers)

**Provider Error Patterns** (following existing providers):
```go
providerErr := &providers.ProviderError{
    Provider: "awsec2",
    Code:     "invalid_credentials",
    Message:  "Invalid AWS credentials",
    Details:  map[string]interface{}{"error": err.Error()},
}
```

### 1.2 AWS EC2 Configuration

**File**: `pkg/config/config.go`

**Tasks**:
- [ ] Add `AWSEC2Config` struct:
  ```go
  type AWSEC2Config struct {
      InstanceID   string `json:"instance_id,omitempty"`
      IP           string `json:"ip"`
      SSHKey       string `json:"ssh_key"`
      SSHKeyName   string `json:"ssh_key_name,omitempty"`
      Username     string `json:"username"`
      Region       string `json:"region,omitempty"`
      InstanceType string `json:"instance_type,omitempty"`
      Provisioned  bool   `json:"provisioned,omitempty"`
  }
  ```
- [ ] Implement `ProviderConfig` interface methods (GetIP, GetUsername, GetSSHKey, IsProvisioned)
- [ ] Add `GetAWSEC2Config() (*AWSEC2Config, error)` convenience method to `TargetConfig`
- [ ] Update `GetSSHProviderConfig()` to include `case "awsec2"`
- [ ] Update `GetAnyProviderConfig()` to include `case "awsec2"`

### 1.3 AWS Credential Storage

**File**: `pkg/config/config.go` (tokens)

**Tasks**:
- [ ] Store AWS credentials as JSON in tokens.json:
  ```json
  {
    "awsec2": "{\"access_key\":\"AKIAIOSFODNN7EXAMPLE\",\"secret_key\":\"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\",\"region\":\"us-east-1\"}"
  }
  ```
- [ ] Add helper methods for AWS credentials:
  - [ ] `SetAWSCredentials(accessKey, secretKey, defaultRegion string)`
  - [ ] `GetAWSCredentials() (accessKey, secretKey, defaultRegion string, err error)`
  - [ ] `HasAWSCredentials() bool`

**Note**: Keep secrets in `tokens.json` (0600 permissions), NOT in main config

### 1.4 EC2-Specific Helpers

**File**: `pkg/providers/awsec2/helpers.go`

**Tasks**:
- [ ] Implement `convertInstanceToServer(*types.Instance) *providers.Server`
- [ ] Implement `convertRegionToProviderRegion(region string) providers.Region`
- [ ] Implement `convertInstanceTypeToSize(types.InstanceTypeInfo) providers.Size`
- [ ] Implement `getDefaultUbuntuAMI(region string) string` (AMI lookup by region)
- [ ] Implement static fallback regions/sizes if API fails (following Vultr pattern)

### 1.5 IP Recovery for EC2

**File**: `cmd/common.go`

**Tasks**:
- [ ] Add EC2 IP recovery logic in `configureTarget()`:
  ```go
  if target.Provider == "awsec2" {
      if ec2Config, ok := target.ProviderConfig["awsec2"].(*config.AWSEC2Config); ok {
          instanceID := ec2Config.InstanceID
          if instanceID == "" {
              if targetState, err := state.GetTargetState(targetName); err == nil {
                  instanceID = targetState.ProvisionedID
              }
          }
          if ec2Config.IP == "" && instanceID != "" {
              fmt.Println("Recovering instance IP from AWS EC2...")
              if err := recoverIPFromAWSEC2(&target, targetName, instanceID); err != nil {
                  return fmt.Errorf("failed to recover IP: %w", err)
              }
          }
      }
  }
  ```
- [ ] Implement `recoverIPFromAWSEC2(target, targetName, instanceID)` helper

---

## Phase 2: AWS S3 Provider (Static Sites Only)

### 2.1 Static Site Detection Enhancement

**File**: `pkg/detector/types.go`

**Tasks**:
- [ ] Add `IsStatic() bool` method to `Detection` type:
  ```go
  func (d *Detection) IsStatic() bool {
      // Check meta for static indicators
      if d.Meta["static"] == "true" {
          return true
      }
      if d.Meta["export"] == "static" {
          return true
      }
      // Check for static-only frameworks
      staticFrameworks := []string{"Jekyll", "Hugo", "Gatsby (static)"}
      for _, fw := range staticFrameworks {
          if d.Framework == fw {
              return true
          }
      }
      // Check run plan for static site indicators
      for _, cmd := range d.RunPlan {
          if strings.Contains(cmd, "Static site") ||
             strings.Contains(cmd, "serve with nginx or CDN") {
              return true
          }
      }
      return false
  }
  ```

**File**: `pkg/detector/detector.go`

**Tasks**:
- [ ] No changes needed (metadata already set by plan functions)

**File**: `pkg/detector/plans/javascript.go`

**Tasks**:
- [ ] Verify all static exports properly set `meta["static"] = "true"`:
  - [ ] Next.js export mode
  - [ ] Astro static mode
  - [ ] SvelteKit static adapter
  - [ ] Gatsby
  - [ ] Any other static-capable frameworks

### 2.2 AWS S3 Client Implementation

**File**: `pkg/providers/awss3/client.go`

**Tasks**:
- [ ] Create `pkg/providers/awss3/` directory
- [ ] Implement `Client` struct with AWS SDK v2 S3 client
- [ ] Add `init()` function to register provider as `"awss3"`
- [ ] Implement **limited** `Provider` interface:
  - [ ] `Name() string` → return `"awss3"`
  - [ ] `DisplayName() string` → return `"AWS S3 (Static Sites)"`
  - [ ] `SupportsProvisioning() bool` → return `true` (provisions bucket)
  - [ ] `SupportsBYOS() bool` → return `false` (S3 doesn't support BYOS)
  - [ ] `ValidateCredentials(ctx) error`
  - [ ] `GetRegions(ctx) ([]Region, error)` → S3 regions
  - [ ] `GetSizes(ctx, region) ([]Size, error)` → return `[]Size{}` (not applicable)
  - [ ] `GetImages(ctx) ([]Image, error)` → return `[]Image{}` (not applicable)
  - [ ] `Provision(ctx, config) (*Server, error)` → provision S3 bucket
  - [ ] `GetServer(ctx, serverID) (*Server, error)` → get bucket info
  - [ ] `Destroy(ctx, serverID) error` → delete bucket
  - [ ] `WaitForActive(ctx, serverID, timeout) (*Server, error)` → wait for bucket
  - [ ] `UploadSSHKey(ctx, name, publicKey) (*SSHKey, error)` → return error (not applicable)

**Dependencies**:
```go
import (
    "github.com/aws/aws-sdk-go-v2/service/s3"
)
```

**Key Implementation Details**:
- S3 bucket provisioning creates public static website hosting
- Enable static website hosting on bucket
- Set bucket policy for public read access
- `serverID` is actually `bucketName` for S3 provider
- `Server.PublicIPv4` stores bucket website endpoint
- No SSH operations (return errors for SSH-related methods)

### 2.3 S3 Configuration

**File**: `pkg/config/config.go`

**Tasks**:
- [ ] Update existing `S3Config` struct:
  ```go
  type S3Config struct {
      Bucket        string `json:"bucket"`
      Region        string `json:"region"`
      WebsiteURL    string `json:"website_url,omitempty"`    // Bucket website endpoint
      Provisioned   bool   `json:"provisioned,omitempty"`
  }
  ```
- [ ] Update `S3Config` methods:
  - [ ] `GetIP() string` → return `WebsiteURL`
  - [ ] `IsProvisioned() bool` → return `Provisioned`

### 2.4 Static Deployment Executor

**File**: `pkg/deploy/s3_executor.go` (new file)

**Tasks**:
- [ ] Create S3-specific deployment executor
- [ ] Implement `DeployToS3(ctx, target, detection, options) error`:
  - [ ] Validate detection is static site (`detection.IsStatic()`)
  - [ ] Resolve builder using existing `resolveBuilder()` logic (flag > config > auto-detect)
  - [ ] Build project locally using builder system (`pkg/builders/`)
  - [ ] Determine build output directory from `detection.Meta["build_output"]` or builder output
  - [ ] Sync build output to S3 bucket using `aws s3 sync`
  - [ ] Return error with clear message if site is not static

**Note**: S3 deployments should support all three builders (native, nixpacks, dockerfile) as long as the output is static files

**Validation Example**:
```go
if !detection.IsStatic() {
    return fmt.Errorf("AWS S3 provider only supports static sites. This project (%s) requires a server. Use AWS EC2 or another server-based provider instead.", detection.Framework)
}
```

### 2.5 Integration with Deploy Command

**File**: `cmd/push.go` and `cmd/deploy.go`

**Tasks**:
- [ ] Add provider type check in push/deploy commands:
  ```go
  if target.Provider == "awss3" {
      // Use S3 deployment executor
      return s3_executor.DeployToS3(ctx, target, detection, deployOpts)
  } else {
      // Use existing SSH-based executor
      return executor.Deploy(ctx, sshExecutor, target, detection, deployOpts)
  }
  ```
- [ ] Show appropriate error if user tries to deploy non-static site to S3

---

## Phase 3: Command Integration

### 3.1 Create Command Updates

**File**: `cmd/create.go`

**Tasks**:
- [ ] Add `awsec2` and `awss3` to provider selection menu
- [ ] Add AWS credential prompts:
  - [ ] Access Key ID
  - [ ] Secret Access Key
  - [ ] Default Region
- [ ] For `awss3`, validate static site before proceeding:
  ```go
  if provider == "awss3" {
      detection := detector.DetectFramework(projectPath)
      if !detection.IsStatic() {
          fmt.Printf("Error: AWS S3 only supports static sites.\n")
          fmt.Printf("Your project (%s) requires a server.\n", detection.Framework)
          fmt.Printf("Please choose AWS EC2 or another server-based provider.\n")
          os.Exit(1)
      }
  }
  ```
- [ ] Update provider-specific config prompts for EC2 (instance type, region)
- [ ] Update provider-specific config prompts for S3 (bucket name, region)

### 3.2 Status Command Updates

**File**: `cmd/status.go`

**Tasks**:
- [ ] Handle S3 targets differently (no SSH connection):
  - [ ] Show bucket status instead of server status
  - [ ] Display website URL
  - [ ] Skip server uptime/health checks (not applicable)
- [ ] EC2 targets use existing SSH-based status logic

### 3.3 SSH Command Updates

**File**: `cmd/ssh.go`

**Tasks**:
- [ ] Block SSH command for S3 targets:
  ```go
  if target.Provider == "awss3" {
      fmt.Println("Error: SSH is not available for S3 static hosting.")
      fmt.Println("S3 is a managed storage service with no server access.")
      os.Exit(1)
  }
  ```

### 3.4 Logs Command Updates

**File**: `cmd/logs.go`

**Tasks**:
- [ ] Block logs command for S3 targets:
  ```go
  if target.Provider == "awss3" {
      fmt.Println("Error: Application logs are not available for S3 static hosting.")
      fmt.Println("Enable S3 access logs for request tracking if needed.")
      os.Exit(1)
  }
  ```

### 3.5 Rollback Command Updates

**File**: `cmd/rollback.go`

**Tasks**:
- [ ] Implement S3 versioning-based rollback (optional):
  - [ ] Enable S3 versioning on bucket
  - [ ] Track previous deployment versions
  - [ ] Rollback by restoring previous object versions
- [ ] OR block rollback for S3 with clear message

---

## Phase 4: Testing

### 4.1 AWS EC2 Provider Tests

**File**: `test/providers/awsec2_test.go`

**Tasks**:
- [ ] Test EC2 client creation
- [ ] Test provider interface compliance
- [ ] Test credential validation
- [ ] Test region listing
- [ ] Test instance type (size) listing
- [ ] Test AMI (image) listing
- [ ] Test provision config validation
- [ ] Test server conversion helpers
- [ ] Test error handling with `ProviderError`
- [ ] Test context cancellation handling
- [ ] Mock AWS SDK calls using `aws-sdk-go-v2` test helpers

**Pattern**: Follow `test/providers/digitalocean_test.go` structure

### 4.2 AWS S3 Provider Tests

**File**: `test/providers/awss3_test.go`

**Tasks**:
- [ ] Test S3 client creation
- [ ] Test provider interface compliance (limited methods)
- [ ] Test credential validation
- [ ] Test bucket provisioning
- [ ] Test static site validation (reject non-static deployments)
- [ ] Test bucket deletion
- [ ] Test error handling
- [ ] Mock AWS SDK S3 calls

### 4.3 Static Detection Tests

**File**: `test/detector/static_detection_test.go` (new)

**Tasks**:
- [ ] Test `Detection.IsStatic()` method:
  - [ ] Next.js export mode → true
  - [ ] Next.js standalone mode → false
  - [ ] Astro static → true
  - [ ] Astro SSR → false
  - [ ] Jekyll → true
  - [ ] Hugo → true
  - [ ] SvelteKit static adapter → true
  - [ ] SvelteKit node adapter → false
  - [ ] Express.js → false
  - [ ] FastAPI → false
- [ ] Use dynamic test project creation pattern

### 4.4 S3 Deployment Tests

**File**: `test/deploy/s3_deployment_test.go` (new)

**Tasks**:
- [ ] Test S3 deployment executor
- [ ] Test static site validation
- [ ] Test build output directory detection
- [ ] Test rejection of non-static sites
- [ ] Mock S3 sync operations

### 4.5 Integration Tests

**File**: `test/integration/aws_integration_test.go` (new)

**Tasks**:
- [ ] Test EC2 provider registration
- [ ] Test S3 provider registration
- [ ] Test provider selection based on static detection
- [ ] Test config serialization/deserialization for both providers
- [ ] Test IP recovery for EC2
- [ ] Test full deployment flow (mock AWS APIs)

---

## Phase 5: Documentation Updates

### 5.1 AGENTS.md Updates

**File**: `AGENTS.md`

**Tasks**:
- [ ] Add AWS EC2 to supported providers list
- [ ] Add AWS S3 to supported providers list (static only)
- [ ] Document static site detection logic
- [ ] Update provider architecture section
- [ ] Add AWS-specific configuration examples
- [ ] Document S3 vs EC2 decision logic

### 5.2 README.md Updates

**File**: `README.md`

**Tasks**:
- [ ] Add AWS to provider list
- [ ] Add note about S3 static-only limitation
- [ ] Update feature list to mention static site detection
- [ ] Add AWS credential setup instructions
- [ ] Add example commands for AWS deployments

---

## Implementation Checklist Summary

### Core Implementation
- [ ] **Phase 1**: AWS EC2 Provider (dynamic sites)
  - [ ] Client implementation (`pkg/providers/awsec2/client.go`)
  - [ ] Config structures (`pkg/config/config.go`)
  - [ ] AWS credential storage (tokens.json)
  - [ ] Helper functions (`pkg/providers/awsec2/helpers.go`)
  - [ ] IP recovery logic (`cmd/common.go`)

- [ ] **Phase 2**: AWS S3 Provider (static sites)
  - [ ] Static detection enhancement (`pkg/detector/types.go`)
  - [ ] S3 client implementation (`pkg/providers/awss3/client.go`)
  - [ ] S3 config updates (`pkg/config/config.go`)
  - [ ] S3 deployment executor (`pkg/deploy/s3_executor.go`)

- [ ] **Phase 3**: Command Integration
  - [ ] Create command (`cmd/create.go`)
  - [ ] Status command (`cmd/status.go`)
  - [ ] SSH command (`cmd/ssh.go`)
  - [ ] Logs command (`cmd/logs.go`)
  - [ ] Rollback command (`cmd/rollback.go`)
  - [ ] Deploy/Push commands (`cmd/deploy.go`, `cmd/push.go`)

- [ ] **Phase 4**: Testing
  - [ ] EC2 provider tests (`test/providers/awsec2_test.go`)
  - [ ] S3 provider tests (`test/providers/awss3_test.go`)
  - [ ] Static detection tests (`test/detector/static_detection_test.go`)
  - [ ] S3 deployment tests (`test/deploy/s3_deployment_test.go`)
  - [ ] Integration tests (`test/integration/aws_integration_test.go`)

- [ ] **Phase 5**: Documentation
  - [ ] Update AGENTS.md
  - [ ] Update README.md

---

## Design Patterns to Follow

### 1. Provider Registration Pattern
```go
// In pkg/providers/awsec2/client.go
func init() {
    providers.Register("awsec2", func(token string) providers.Provider {
        return NewClient(token)
    })
}
```

### 2. Error Handling Pattern
```go
if err != nil {
    return &providers.ProviderError{
        Provider: "awsec2",
        Code:     "create_instance_failed",
        Message:  "Failed to create EC2 instance",
        Details:  map[string]interface{}{"error": err.Error()},
    }
}
```

### 3. Config Storage Pattern
```go
// Store provider-specific config under correct key
target.SetProviderConfig("awsec2", ec2Config)

// Retrieve provider-specific config
ec2Config, err := target.GetAWSEC2Config()
```

### 4. Static Detection Pattern
```go
detection := detector.DetectFramework(projectPath)
if detection.IsStatic() {
    // Use S3 provider
} else {
    // Use EC2 or other server provider
}
```

### 5. Test Pattern (Dynamic Project Creation)
```go
projectPath := createTestProject(t, map[string]string{
    "package.json": `{"dependencies": {"next": "^13.0.0"}}`,
    "next.config.js": `module.exports = { output: 'export' }`,
})
detection := detector.DetectFramework(projectPath)
if !detection.IsStatic() {
    t.Error("Expected static site detection")
}
```

---

## AWS-Specific Considerations

### EC2 AMI Selection
- Default to **Ubuntu 22.04 LTS** (consistent with other providers)
- AMI IDs vary by region - maintain static lookup table or query AWS API
- Fallback to well-known AMI ID if API lookup fails

### EC2 Instance Types
- Filter to minimum 512MB RAM (consistent with DO/Vultr/Hetzner)
- Recommend **t3.micro** or **t3.small** for small apps
- Map instance types to Size struct (Memory, VCPUs, Disk, Price)

### EC2 Security Groups
- Auto-create security group for each deployment
- Open ports: 22 (SSH), 80 (HTTP), 443 (HTTPS)
- Tag security group with target name for cleanup

### S3 Bucket Naming
- Bucket names must be globally unique
- Suggest format: `lightfold-{target-name}-{random-suffix}`
- Validate bucket name against AWS S3 rules (lowercase, no underscores, etc.)

### S3 Website Hosting
- Enable static website hosting on bucket
- Set index document to `index.html`
- Set error document to `404.html` or `index.html` (for SPA routing)
- Bucket policy must allow public `s3:GetObject` for all objects

### AWS Credential Security
- Store access key + secret key as JSON in `tokens.json` (0600 permissions)
- Never log credentials or include in error messages
- Support IAM role credentials if running on EC2 (future enhancement)
- Recommend using AWS IAM user with minimal permissions

---

## Testing Strategy

### Unit Tests
- Test each provider method independently
- Mock AWS SDK calls using `aws-sdk-go-v2` mocking utilities
- Test error conditions and edge cases
- Verify proper error wrapping with `ProviderError`

### Integration Tests
- Test provider registration and retrieval
- Test config serialization/deserialization
- Test full deployment flow with mocked AWS APIs
- Test static site detection and provider selection

### Manual Testing Checklist
- [ ] EC2 instance provisioning end-to-end
- [ ] S3 bucket provisioning end-to-end
- [ ] Static site deployment to S3
- [ ] Dynamic site deployment to EC2
- [ ] IP recovery after config corruption
- [ ] Rollback functionality (EC2)
- [ ] Credential validation
- [ ] Error messages are clear and actionable

---

## Notes

- This implementation maintains **100% consistency** with existing provider patterns
- All code follows **DRY principles** - reusing existing utilities and patterns
- S3 provider has **built-in validation** to reject non-static sites with clear errors
- EC2 provider is **feature-complete** matching DO/Vultr/Hetzner capabilities
- Tests follow **dynamic project creation** pattern used elsewhere in the codebase
- Documentation updates ensure users understand **when to use EC2 vs S3**

---

## Session Workflow

This document is designed to be used across **multiple coding sessions**. Each session should:

1. Pick a specific phase or task from the checklist
2. Mark tasks as completed using `[x]` checkboxes
3. Commit changes with clear commit messages
4. Update this document with any new insights or design decisions
5. Run tests before moving to the next phase

**Estimated Implementation Time**: 3-5 sessions (depending on AWS SDK familiarity)
