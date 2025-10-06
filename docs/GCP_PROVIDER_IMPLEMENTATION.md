# GCP Provider Implementation Plan

This document outlines the complete implementation plan for adding GCP Compute Engine (dynamic server provisioning) and GCP Cloud Storage (static site hosting) providers to Lightfold CLI. The implementation follows the existing provider pattern used by DigitalOcean, Vultr, Hetzner, and AWS for consistency and maintainability.

---

## Overview

### Goals
1. **GCP Compute Engine Provider**: Full-featured VPS provisioning for web applications
2. **GCP Cloud Storage Provider**: Static site hosting (static sites only)
3. **Detection Enhancement**: Leverage existing static detection to recommend appropriate provider
4. **Consistent Design**: Follow existing patterns for DRY, maintainable code
5. **Comprehensive Testing**: Match or exceed test coverage of existing providers

### Design Principles
- **DRY (Don't Repeat Yourself)**: Reuse existing patterns and utilities
- **Provider Registry Pattern**: Auto-registration at init time
- **Interface-Based Design**: Implement `providers.Provider` interface
- **Static Site Validation**: Cloud Storage provider rejects non-static deployments
- **IP Recovery**: Compute Engine implements same recovery pattern as DO/Vultr/Hetzner/AWS EC2

---

## Phase 1: GCP Compute Engine Provider (Dynamic Sites)

### 1.1 GCP Compute Engine Client Implementation

**File**: `pkg/providers/gcp/client.go`

**Tasks**:
- [ ] Create `pkg/providers/gcp/` directory
- [ ] Implement `Client` struct with GCP Compute Engine client
- [ ] Add `init()` function to register provider as `"gcp"`
- [ ] Implement `Provider` interface methods:
  - [ ] `Name() string` → return `"gcp"`
  - [ ] `DisplayName() string` → return `"GCP Compute Engine"`
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
    "google.golang.org/api/compute/v1"
    "google.golang.org/api/option"
    "golang.org/x/oauth2/google"
)
```

**Key Implementation Details**:
- Use Google Cloud Compute Engine API v1
- Handle GCP credentials via service account JSON key (stored in tokens.json as base64-encoded string)
- Map Compute Engine instance states to provider status strings
- Convert GCP API responses to `providers.Server` struct
- Implement error wrapping with `providers.ProviderError`
- Default to Ubuntu 22.04 LTS image when listing images
- Filter machine types to minimum 512MB RAM (like other providers)
- Auto-create firewall rules for HTTP/HTTPS/SSH
- Use default VPC network or create dedicated network per target

**Provider Error Patterns** (following existing providers):
```go
providerErr := &providers.ProviderError{
    Provider: "gcp",
    Code:     "invalid_credentials",
    Message:  "Invalid GCP service account credentials",
    Details:  map[string]interface{}{"error": err.Error()},
}
```

**GCP-Specific Patterns**:
- Project ID is required for all API calls
- Zone format: `{region}-{zone}` (e.g., `us-central1-a`)
- Instance names must match regex: `[a-z][-a-z0-9]*`
- SSH keys are instance metadata, not account-level resources

### 1.2 GCP Compute Engine Configuration

**File**: `pkg/config/config.go`

**Tasks**:
- [ ] Add `GCPConfig` struct:
  ```go
  type GCPConfig struct {
      InstanceID   string `json:"instance_id,omitempty"`    // Full resource path
      InstanceName string `json:"instance_name,omitempty"`  // Short name
      IP           string `json:"ip"`
      SSHKey       string `json:"ssh_key"`
      SSHKeyName   string `json:"ssh_key_name,omitempty"`
      Username     string `json:"username"`
      ProjectID    string `json:"project_id"`
      Zone         string `json:"zone,omitempty"`           // e.g., us-central1-a
      MachineType  string `json:"machine_type,omitempty"`   // e.g., e2-micro
      Provisioned  bool   `json:"provisioned,omitempty"`
  }
  ```
- [ ] Implement `ProviderConfig` interface methods (GetIP, GetUsername, GetSSHKey, IsProvisioned)
- [ ] Add `GetGCPConfig() (*GCPConfig, error)` convenience method to `TargetConfig`
- [ ] Update `GetSSHProviderConfig()` to include `case "gcp"`
- [ ] Update `GetAnyProviderConfig()` to include `case "gcp"`

### 1.3 GCP Credential Storage

**File**: `pkg/config/config.go` (tokens)

**Tasks**:
- [ ] Store GCP service account JSON as base64-encoded string in tokens.json:
  ```json
  {
    "gcp": "{\"type\":\"service_account\",\"project_id\":\"my-project-123\",\"private_key_id\":\"abc123...\",\"private_key\":\"-----BEGIN PRIVATE KEY-----\\n...\",\"client_email\":\"lightfold@my-project.iam.gserviceaccount.com\",\"client_id\":\"123456789\",\"auth_uri\":\"https://accounts.google.com/o/oauth2/auth\",\"token_uri\":\"https://oauth2.googleapis.com/token\",\"auth_provider_x509_cert_url\":\"https://www.googleapis.com/oauth2/v1/certs\",\"client_x509_cert_url\":\"https://www.googleapis.com/robot/v1/metadata/x509/...\"}"
  }
  ```
- [ ] Add helper methods for GCP credentials:
  - [ ] `SetGCPServiceAccount(serviceAccountJSON string)`
  - [ ] `GetGCPServiceAccount() (string, error)`
  - [ ] `HasGCPServiceAccount() bool`
  - [ ] `GetGCPProjectID() (string, error)` - extract project_id from service account JSON

**Note**: Keep service account JSON in `tokens.json` (0600 permissions), NOT in main config

### 1.4 GCP-Specific Helpers

**File**: `pkg/providers/gcp/helpers.go`

**Tasks**:
- [ ] Implement `convertInstanceToServer(*compute.Instance, projectID, zone) *providers.Server`
- [ ] Implement `convertZoneToRegion(zone string) string` (e.g., `us-central1-a` → `us-central1`)
- [ ] Implement `convertMachineTypeToSize(*compute.MachineType) providers.Size`
- [ ] Implement `getDefaultUbuntuImage(projectID string) (string, error)` - query for latest Ubuntu 22.04 LTS
- [ ] Implement static fallback zones/machine types if API fails (following Vultr pattern)
- [ ] Implement `parseInstanceID(instanceID string) (projectID, zone, name string, err error)` - parse full resource path
- [ ] Implement `formatInstanceID(projectID, zone, name string) string` - create full resource path

**Resource Path Format**:
```
projects/{project}/zones/{zone}/instances/{name}
```

### 1.5 IP Recovery for GCP

**File**: `cmd/common.go`

**Tasks**:
- [ ] Add GCP IP recovery logic in `configureTarget()`:
  ```go
  if target.Provider == "gcp" {
      if gcpConfig, ok := target.ProviderConfig["gcp"].(*config.GCPConfig); ok {
          instanceID := gcpConfig.InstanceID
          if instanceID == "" {
              if targetState, err := state.GetTargetState(targetName); err == nil {
                  instanceID = targetState.ProvisionedID
              }
          }
          if gcpConfig.IP == "" && instanceID != "" {
              fmt.Println("Recovering instance IP from GCP Compute Engine...")
              if err := recoverIPFromGCP(&target, targetName, instanceID); err != nil {
                  return fmt.Errorf("failed to recover IP: %w", err)
              }
          }
      }
  }
  ```
- [ ] Implement `recoverIPFromGCP(target, targetName, instanceID)` helper

### 1.6 GCP Firewall Rules

**File**: `pkg/providers/gcp/firewall.go`

**Tasks**:
- [ ] Implement `ensureFirewallRules(projectID, network string) error`:
  - [ ] Create firewall rule for SSH (port 22)
  - [ ] Create firewall rule for HTTP (port 80)
  - [ ] Create firewall rule for HTTPS (port 443)
  - [ ] Tag rules with `lightfold-managed` for easy identification
  - [ ] Skip creation if rules already exist
- [ ] Implement `deleteFirewallRules(projectID string) error` - cleanup on destroy

**Firewall Rule Naming**:
```
lightfold-allow-ssh
lightfold-allow-http
lightfold-allow-https
```

### 1.7 GCP SSH Key Management

**File**: `pkg/providers/gcp/client.go`

**Tasks**:
- [ ] Implement `UploadSSHKey()` differently than other providers:
  - GCP stores SSH keys as instance metadata, not account-level
  - Format: `{username}:{ssh_public_key}`
  - Store key data for use during instance provisioning
  - Return pseudo `SSHKey` struct for interface compliance
- [ ] Include SSH key in instance metadata during `Provision()`

---

## Phase 2: GCP Cloud Storage Provider (Static Sites Only)

### 2.1 Static Site Detection Enhancement

**File**: `pkg/detector/types.go`

**Tasks**:
- [ ] **No changes needed** - `IsStatic()` method will be added as part of AWS implementation
- [ ] Verify method exists and works correctly for GCP use case

### 2.2 GCP Cloud Storage Client Implementation

**File**: `pkg/providers/gcpstorage/client.go`

**Tasks**:
- [ ] Create `pkg/providers/gcpstorage/` directory
- [ ] Implement `Client` struct with GCP Cloud Storage client
- [ ] Add `init()` function to register provider as `"gcpstorage"`
- [ ] Implement **limited** `Provider` interface:
  - [ ] `Name() string` → return `"gcpstorage"`
  - [ ] `DisplayName() string` → return `"GCP Cloud Storage (Static Sites)"`
  - [ ] `SupportsProvisioning() bool` → return `true` (provisions bucket)
  - [ ] `SupportsBYOS() bool` → return `false` (Cloud Storage doesn't support BYOS)
  - [ ] `ValidateCredentials(ctx) error`
  - [ ] `GetRegions(ctx) ([]Region, error)` → Cloud Storage regions/locations
  - [ ] `GetSizes(ctx, region) ([]Size, error)` → return `[]Size{}` (not applicable)
  - [ ] `GetImages(ctx) ([]Image, error)` → return `[]Image{}` (not applicable)
  - [ ] `Provision(ctx, config) (*Server, error)` → provision bucket
  - [ ] `GetServer(ctx, serverID) (*Server, error)` → get bucket info
  - [ ] `Destroy(ctx, serverID) error` → delete bucket
  - [ ] `WaitForActive(ctx, serverID, timeout) (*Server, error)` → wait for bucket
  - [ ] `UploadSSHKey(ctx, name, publicKey) (*SSHKey, error)` → return error (not applicable)

**Dependencies**:
```go
import (
    "google.golang.org/api/storage/v1"
)
```

**Key Implementation Details**:
- Cloud Storage bucket provisioning creates public static website hosting
- Enable website configuration on bucket (index/404 pages)
- Set bucket ACL for public read access (`allUsers` reader role)
- `serverID` is actually `bucketName` for Cloud Storage provider
- `Server.PublicIPv4` stores bucket website endpoint
- No SSH operations (return errors for SSH-related methods)
- Project ID required for all API calls (extract from service account)

### 2.3 GCP Cloud Storage Configuration

**File**: `pkg/config/config.go`

**Tasks**:
- [ ] Add `GCPStorageConfig` struct:
  ```go
  type GCPStorageConfig struct {
      Bucket          string `json:"bucket"`
      ProjectID       string `json:"project_id"`
      Location        string `json:"location"`              // e.g., US, EU, ASIA
      WebsiteURL      string `json:"website_url,omitempty"` // Bucket website endpoint
      Provisioned     bool   `json:"provisioned,omitempty"`
  }
  ```
- [ ] Implement `ProviderConfig` interface methods:
  - [ ] `GetIP() string` → return `WebsiteURL`
  - [ ] `GetUsername() string` → return `""` (not applicable)
  - [ ] `GetSSHKey() string` → return `""` (not applicable)
  - [ ] `IsProvisioned() bool` → return `Provisioned`
- [ ] Add `GetGCPStorageConfig() (*GCPStorageConfig, error)` convenience method to `TargetConfig`
- [ ] Update `GetAnyProviderConfig()` to include `case "gcpstorage"`

### 2.4 Static Deployment Executor for GCP

**File**: `pkg/deploy/gcpstorage_executor.go` (new file)

**Tasks**:
- [ ] Create GCP Cloud Storage-specific deployment executor
- [ ] Implement `DeployToGCPStorage(ctx, target, detection, options) error`:
  - [ ] Validate detection is static site (`detection.IsStatic()`)
  - [ ] Resolve builder using existing `resolveBuilder()` logic (flag > config > auto-detect)
  - [ ] Build project locally using builder system (`pkg/builders/`)
  - [ ] Determine build output directory from `detection.Meta["build_output"]` or builder output
  - [ ] Sync build output to Cloud Storage bucket using `gsutil rsync` or SDK
  - [ ] Set proper content types for files (HTML, CSS, JS, images, etc.)
  - [ ] Return error with clear message if site is not static

**Note**: Cloud Storage deployments should support all three builders (native, nixpacks, dockerfile) as long as the output is static files

**Validation Example**:
```go
if !detection.IsStatic() {
    return fmt.Errorf("GCP Cloud Storage provider only supports static sites. This project (%s) requires a server. Use GCP Compute Engine or another server-based provider instead.", detection.Framework)
}
```

### 2.5 Integration with Deploy Command

**File**: `cmd/push.go` and `cmd/deploy.go`

**Tasks**:
- [ ] Add provider type check in push/deploy commands:
  ```go
  if target.Provider == "gcpstorage" {
      // Use GCP Cloud Storage deployment executor
      return gcpstorage_executor.DeployToGCPStorage(ctx, target, detection, deployOpts)
  } else if target.Provider == "awss3" {
      // Use S3 deployment executor
      return s3_executor.DeployToS3(ctx, target, detection, deployOpts)
  } else {
      // Use existing SSH-based executor
      return executor.Deploy(ctx, sshExecutor, target, detection, deployOpts)
  }
  ```
- [ ] Show appropriate error if user tries to deploy non-static site to Cloud Storage

---

## Phase 3: Command Integration

### 3.1 Create Command Updates

**File**: `cmd/create.go`

**Tasks**:
- [ ] Add `gcp` and `gcpstorage` to provider selection menu
- [ ] Add GCP credential prompts:
  - [ ] Service Account JSON (paste full JSON or provide file path)
  - [ ] Project ID (auto-extracted from service account, but confirm)
  - [ ] Default Zone (for Compute Engine)
  - [ ] Default Location (for Cloud Storage)
- [ ] For `gcpstorage`, validate static site before proceeding:
  ```go
  if provider == "gcpstorage" {
      detection := detector.DetectFramework(projectPath)
      if !detection.IsStatic() {
          fmt.Printf("Error: GCP Cloud Storage only supports static sites.\n")
          fmt.Printf("Your project (%s) requires a server.\n", detection.Framework)
          fmt.Printf("Please choose GCP Compute Engine or another server-based provider.\n")
          os.Exit(1)
      }
  }
  ```
- [ ] Update provider-specific config prompts for GCP (machine type, zone, network)
- [ ] Update provider-specific config prompts for Cloud Storage (bucket name, location)

### 3.2 Status Command Updates

**File**: `cmd/status.go`

**Tasks**:
- [ ] Handle Cloud Storage targets differently (no SSH connection):
  - [ ] Show bucket status instead of server status
  - [ ] Display website URL
  - [ ] Skip server uptime/health checks (not applicable)
- [ ] GCP Compute Engine targets use existing SSH-based status logic

### 3.3 SSH Command Updates

**File**: `cmd/ssh.go`

**Tasks**:
- [ ] Block SSH command for Cloud Storage targets:
  ```go
  if target.Provider == "gcpstorage" {
      fmt.Println("Error: SSH is not available for Cloud Storage static hosting.")
      fmt.Println("Cloud Storage is a managed storage service with no server access.")
      os.Exit(1)
  }
  ```

### 3.4 Logs Command Updates

**File**: `cmd/logs.go`

**Tasks**:
- [ ] Block logs command for Cloud Storage targets:
  ```go
  if target.Provider == "gcpstorage" {
      fmt.Println("Error: Application logs are not available for Cloud Storage static hosting.")
      fmt.Println("Enable Cloud Storage access logging for request tracking if needed.")
      os.Exit(1)
  }
  ```

### 3.5 Rollback Command Updates

**File**: `cmd/rollback.go`

**Tasks**:
- [ ] Implement Cloud Storage versioning-based rollback (optional):
  - [ ] Enable object versioning on bucket
  - [ ] Track previous deployment versions
  - [ ] Rollback by restoring previous object versions
- [ ] OR block rollback for Cloud Storage with clear message

---

## Phase 4: Testing

### 4.1 GCP Compute Engine Provider Tests

**File**: `test/providers/gcp_test.go`

**Tasks**:
- [ ] Test GCP client creation with service account JSON
- [ ] Test provider interface compliance
- [ ] Test credential validation
- [ ] Test zone/region listing
- [ ] Test machine type (size) listing
- [ ] Test image listing (Ubuntu focus)
- [ ] Test provision config validation
- [ ] Test instance conversion helpers
- [ ] Test instance ID parsing/formatting
- [ ] Test firewall rule creation
- [ ] Test error handling with `ProviderError`
- [ ] Test context cancellation handling
- [ ] Mock GCP Compute API calls

**Pattern**: Follow `test/providers/digitalocean_test.go` structure

### 4.2 GCP Cloud Storage Provider Tests

**File**: `test/providers/gcpstorage_test.go`

**Tasks**:
- [ ] Test Cloud Storage client creation
- [ ] Test provider interface compliance (limited methods)
- [ ] Test credential validation
- [ ] Test bucket provisioning
- [ ] Test static site validation (reject non-static deployments)
- [ ] Test bucket deletion
- [ ] Test error handling
- [ ] Mock GCP Cloud Storage API calls

### 4.3 Static Detection Tests

**File**: `test/detector/static_detection_test.go`

**Tasks**:
- [ ] **Already covered by AWS implementation** - verify GCP use cases work
- [ ] Add GCP-specific edge cases if needed

### 4.4 GCP Cloud Storage Deployment Tests

**File**: `test/deploy/gcpstorage_deployment_test.go` (new)

**Tasks**:
- [ ] Test GCP Cloud Storage deployment executor
- [ ] Test static site validation
- [ ] Test build output directory detection
- [ ] Test rejection of non-static sites
- [ ] Test content type setting for various file types
- [ ] Mock Cloud Storage sync operations

### 4.5 Integration Tests

**File**: `test/integration/gcp_integration_test.go` (new)

**Tasks**:
- [ ] Test GCP provider registration
- [ ] Test Cloud Storage provider registration
- [ ] Test provider selection based on static detection
- [ ] Test config serialization/deserialization for both providers
- [ ] Test IP recovery for Compute Engine
- [ ] Test full deployment flow (mock GCP APIs)
- [ ] Test service account credential handling

---

## Phase 5: Documentation Updates

### 5.1 AGENTS.md Updates

**File**: `AGENTS.md`

**Tasks**:
- [ ] Add GCP Compute Engine to supported providers list
- [ ] Add GCP Cloud Storage to supported providers list (static only)
- [ ] Document GCP-specific configuration (service account, project ID, zones)
- [ ] Update provider architecture section
- [ ] Add GCP configuration examples
- [ ] Document Cloud Storage vs Compute Engine decision logic

### 5.2 README.md Updates

**File**: `README.md`

**Tasks**:
- [ ] Add GCP to provider list
- [ ] Add note about Cloud Storage static-only limitation
- [ ] Add GCP service account setup instructions
- [ ] Add example commands for GCP deployments
- [ ] Document zone selection for Compute Engine

---

## Implementation Checklist Summary

### Core Implementation
- [ ] **Phase 1**: GCP Compute Engine Provider (dynamic sites)
  - [ ] Client implementation (`pkg/providers/gcp/client.go`)
  - [ ] Config structures (`pkg/config/config.go`)
  - [ ] Service account credential storage (tokens.json)
  - [ ] Helper functions (`pkg/providers/gcp/helpers.go`)
  - [ ] Firewall rule management (`pkg/providers/gcp/firewall.go`)
  - [ ] IP recovery logic (`cmd/common.go`)

- [ ] **Phase 2**: GCP Cloud Storage Provider (static sites)
  - [ ] Static detection enhancement (reuse from AWS implementation)
  - [ ] Cloud Storage client implementation (`pkg/providers/gcpstorage/client.go`)
  - [ ] Cloud Storage config structures (`pkg/config/config.go`)
  - [ ] Cloud Storage deployment executor (`pkg/deploy/gcpstorage_executor.go`)

- [ ] **Phase 3**: Command Integration
  - [ ] Create command (`cmd/create.go`)
  - [ ] Status command (`cmd/status.go`)
  - [ ] SSH command (`cmd/ssh.go`)
  - [ ] Logs command (`cmd/logs.go`)
  - [ ] Rollback command (`cmd/rollback.go`)
  - [ ] Deploy/Push commands (`cmd/deploy.go`, `cmd/push.go`)

- [ ] **Phase 4**: Testing
  - [ ] GCP Compute Engine provider tests (`test/providers/gcp_test.go`)
  - [ ] Cloud Storage provider tests (`test/providers/gcpstorage_test.go`)
  - [ ] Cloud Storage deployment tests (`test/deploy/gcpstorage_deployment_test.go`)
  - [ ] Integration tests (`test/integration/gcp_integration_test.go`)

- [ ] **Phase 5**: Documentation
  - [ ] Update AGENTS.md
  - [ ] Update README.md

---

## Design Patterns to Follow

### 1. Provider Registration Pattern
```go
// In pkg/providers/gcp/client.go
func init() {
    providers.Register("gcp", func(token string) providers.Provider {
        return NewClient(token)
    })
}

// In pkg/providers/gcpstorage/client.go
func init() {
    providers.Register("gcpstorage", func(token string) providers.Provider {
        return NewClient(token)
    })
}
```

### 2. Error Handling Pattern
```go
if err != nil {
    return &providers.ProviderError{
        Provider: "gcp",
        Code:     "create_instance_failed",
        Message:  "Failed to create Compute Engine instance",
        Details:  map[string]interface{}{"error": err.Error()},
    }
}
```

### 3. Config Storage Pattern
```go
// Store provider-specific config under correct key
target.SetProviderConfig("gcp", gcpConfig)

// Retrieve provider-specific config
gcpConfig, err := target.GetGCPConfig()
```

### 4. Service Account Credential Pattern
```go
// Parse service account JSON from tokens
credentials, err := google.CredentialsFromJSON(ctx, []byte(serviceAccountJSON), compute.CloudPlatformScope)
if err != nil {
    return fmt.Errorf("invalid service account credentials: %w", err)
}

// Create compute client
computeService, err := compute.NewService(ctx, option.WithCredentials(credentials))
```

### 5. Static Detection Pattern
```go
detection := detector.DetectFramework(projectPath)
if detection.IsStatic() {
    // Use Cloud Storage provider
} else {
    // Use Compute Engine or other server provider
}
```

### 6. Test Pattern (Dynamic Project Creation)
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

## GCP-Specific Considerations

### Compute Engine Image Selection
- Default to **Ubuntu 22.04 LTS** (consistent with other providers)
- Use image family: `projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts`
- Images are global resources, not zone-specific

### Compute Engine Machine Types
- Filter to minimum 512MB RAM (consistent with DO/Vultr/Hetzner/AWS)
- Recommend **e2-micro** (shared-core, 1 GB RAM, ~$7/month) or **e2-small** (2 GB RAM)
- Machine types are zone-specific (e.g., `zones/us-central1-a/machineTypes/e2-micro`)
- Map machine types to Size struct (Memory, VCPUs, Disk, Price)

### Compute Engine Zones and Regions
- Region format: `us-central1`, `europe-west1`, `asia-east1`
- Zone format: `us-central1-a`, `us-central1-b`, `europe-west1-d`
- Each region has multiple zones (usually 3-4)
- Zone required for instance creation; region used for grouping

### Compute Engine Firewall Rules
- Firewall rules are project-level, not instance-level
- Auto-create rules if they don't exist (idempotent)
- Tag rules with `lightfold-managed` for identification
- Default VPC network name: `default`
- Open ports: 22 (SSH), 80 (HTTP), 443 (HTTPS)

### Compute Engine SSH Keys
- SSH keys stored as instance metadata, not project metadata
- Metadata format: `{username}:{ssh_public_key_data}`
- Keys can be added during instance creation or updated later
- Username must match SSH connection username (usually `root` or `deploy`)

### Cloud Storage Bucket Naming
- Bucket names must be globally unique across all GCP
- Suggest format: `lightfold-{target-name}-{random-suffix}`
- Validate bucket name against GCP Storage rules:
  - 3-63 characters
  - Lowercase letters, numbers, hyphens, underscores, dots
  - Must start and end with letter or number
  - Cannot contain `google` or misspellings

### Cloud Storage Website Hosting
- Enable website configuration with `MainPageSuffix` and `NotFoundPage`
- Set MainPageSuffix to `index.html`
- Set NotFoundPage to `404.html` or `index.html` (for SPA routing)
- Make all objects publicly readable: `allUsers` with `READER` role
- Bucket website endpoint format: `https://storage.googleapis.com/{bucket}/index.html`

### GCP Credential Security
- Store service account JSON in `tokens.json` (0600 permissions)
- Never log credentials or include in error messages
- Service account should have minimal IAM roles:
  - **Compute Engine**: `roles/compute.instanceAdmin.v1`
  - **Cloud Storage**: `roles/storage.admin`
- Consider using Workload Identity if running on GKE (future enhancement)

### GCP Project ID
- Required for all API calls
- Extract from service account JSON: `project_id` field
- Store in config for quick access
- Validate project exists and is accessible during credential validation

---

## Testing Strategy

### Unit Tests
- Test each provider method independently
- Mock GCP API calls using `google.golang.org/api` test utilities
- Test error conditions and edge cases
- Verify proper error wrapping with `ProviderError`
- Test service account JSON parsing and validation

### Integration Tests
- Test provider registration and retrieval
- Test config serialization/deserialization
- Test full deployment flow with mocked GCP APIs
- Test static site detection and provider selection
- Test firewall rule creation and idempotency

### Manual Testing Checklist
- [ ] Compute Engine instance provisioning end-to-end
- [ ] Cloud Storage bucket provisioning end-to-end
- [ ] Static site deployment to Cloud Storage
- [ ] Dynamic site deployment to Compute Engine
- [ ] IP recovery after config corruption
- [ ] Rollback functionality (Compute Engine)
- [ ] Service account credential validation
- [ ] Firewall rule creation and cleanup
- [ ] SSH connection to Compute Engine instance
- [ ] Error messages are clear and actionable

---

## GCP vs AWS Differences

### Authentication
- **AWS**: Access Key + Secret Key
- **GCP**: Service Account JSON (includes private key, project ID, client email)

### Regions/Zones
- **AWS**: Regions (e.g., `us-east-1`), Availability Zones (e.g., `us-east-1a`)
- **GCP**: Regions (e.g., `us-central1`), Zones (e.g., `us-central1-a`)
- **Difference**: GCP requires zone for instance creation; AWS can use region

### Instance Identification
- **AWS EC2**: Instance ID (e.g., `i-1234567890abcdef0`)
- **GCP Compute**: Full resource path (e.g., `projects/my-project/zones/us-central1-a/instances/my-instance`)

### SSH Keys
- **AWS EC2**: Key pairs are region-specific resources
- **GCP Compute**: Keys are instance metadata (per-instance, not account-level)

### Firewall Rules
- **AWS EC2**: Security groups attached to instances
- **GCP Compute**: Firewall rules at project/network level, applied via tags or target instances

### Static Hosting
- **AWS S3**: Public bucket with website hosting enabled
- **GCP Cloud Storage**: Public bucket with website configuration

### Pricing Model
- **AWS EC2**: Hourly billing
- **GCP Compute**: Per-second billing (after first minute)

---

## Notes

- This implementation maintains **100% consistency** with existing provider patterns
- All code follows **DRY principles** - reusing existing utilities and patterns
- Cloud Storage provider has **built-in validation** to reject non-static sites with clear errors
- GCP Compute Engine provider is **feature-complete** matching DO/Vultr/Hetzner/AWS EC2 capabilities
- Tests follow **dynamic project creation** pattern used elsewhere in the codebase
- Documentation updates ensure users understand **when to use Compute Engine vs Cloud Storage**
- **Service account security** is prioritized - JSON stored securely, never logged
- **Firewall rules are idempotent** - safe to run multiple times without duplication
- **Zone-based instance creation** matches GCP's resource model

---

## Session Workflow

This document is designed to be used across **multiple coding sessions**. Each session should:

1. Pick a specific phase or task from the checklist
2. Mark tasks as completed using `[x]` checkboxes
3. Commit changes with clear commit messages
4. Update this document with any new insights or design decisions
5. Run tests before moving to the next phase

**Estimated Implementation Time**: 3-5 sessions (depending on GCP API familiarity)

---

## Future Enhancements (Not in Initial Scope)

- [ ] Cloud Storage object versioning for easy rollback
