# Adding New Cloud Providers to Lightfold

This guide provides a complete, step-by-step process for integrating new cloud providers into Lightfold. Following this guide ensures full compatibility with all Lightfold features including provisioning, deployment, IP recovery, and multi-app server management.

## Table of Contents

1. [Overview](#overview)
2. [Provider Interface](#provider-interface)
3. [Step-by-Step Integration](#step-by-step-integration)
4. [Integration Checklist](#integration-checklist)
5. [Common Pitfalls](#common-pitfalls)
6. [Testing](#testing)
7. [Examples](#examples)

---

## Overview

Lightfold uses a **registry pattern** for providers, making integration straightforward. Each provider:

1. Implements the `Provider` interface
2. Auto-registers itself via `init()` function
3. Integrates with orchestrator, utilities, and command layers

**Time Estimate**: 2-4 hours for a typical VPS provider (like Linode, DigitalOcean)

---

## Provider Interface

All providers must implement this interface (`pkg/providers/provider.go`):

```go
type Provider interface {
    Name() string
    DisplayName() string
    ValidateCredentials(ctx context.Context) error
    Provision(ctx context.Context, config ProvisionConfig) (*Server, error)
    GetServer(ctx context.Context, serverID string) (*Server, error)
    WaitForActive(ctx context.Context, serverID string, timeout time.Duration) (*Server, error)
    DestroyServer(ctx context.Context, serverID string) error
    UploadSSHKey(ctx context.Context, name, publicKey string) (*SSHKey, error)
    SupportsSSH() bool // true for VPS providers, false for container platforms
}
```

---

## Step-by-Step Integration

### Phase 1: Core Provider Implementation

#### 1.1 Create Provider Package

```bash
mkdir -p pkg/providers/newprovider
touch pkg/providers/newprovider/client.go
```

#### 1.2 Implement Client Struct

**File**: `pkg/providers/newprovider/client.go`

```go
package newprovider

import (
    "context"
    "fmt"
    "lightfold/pkg/providers"
    "time"
    // Import provider's official SDK
)

type Client struct {
    token  string
    client *officialsdk.Client // Provider's SDK client
}

func init() {
    // Auto-register provider
    providers.Register("newprovider", func(token string) providers.Provider {
        return NewClient(token)
    })
}

func NewClient(token string) *Client {
    return &Client{
        token:  token,
        client: officialsdk.NewClient(token),
    }
}

func (c *Client) Name() string {
    return "newprovider"
}

func (c *Client) DisplayName() string {
    return "New Provider"
}

func (c *Client) SupportsSSH() bool {
    return true // false for container platforms like Fly.io
}

// Implement remaining interface methods...
```

**Key Implementation Notes**:
- Use the provider's official Go SDK when available
- Handle API rate limits and retries appropriately
- Return proper error messages for common failure cases
- Map provider-specific IDs/names to Lightfold's `Server` struct

#### 1.3 Implement ValidateCredentials

```go
func (c *Client) ValidateCredentials(ctx context.Context) error {
    // Make a lightweight API call to verify token
    // Example: List regions, get account info, etc.
    _, err := c.client.Regions.List(ctx)
    if err != nil {
        return fmt.Errorf("invalid API token or insufficient permissions: %w", err)
    }
    return nil
}
```

#### 1.4 Implement Provision

```go
func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
    // Map Lightfold config to provider's API format
    createRequest := &officialsdk.CreateInstanceRequest{
        Name:   config.Name,
        Region: config.Region,
        Size:   config.Size,
        Image:  config.Image,
        SSHKeys: config.SSHKeys,
        UserData: config.UserData,
        Tags:   config.Tags,
    }

    instance, err := c.client.Instances.Create(ctx, createRequest)
    if err != nil {
        return nil, fmt.Errorf("failed to provision instance: %w", err)
    }

    // Map provider response to Lightfold Server struct
    return &providers.Server{
        ID:          instance.ID,
        Name:        instance.Name,
        PublicIPv4:  instance.PublicIP,
        Region:      instance.Region,
        Status:      instance.Status,
        CreatedAt:   instance.CreatedAt,
        Metadata:    config.Metadata, // Pass through metadata
    }, nil
}
```

#### 1.5 Implement Remaining Interface Methods

- `GetServer()` - Fetch server details by ID
- `WaitForActive()` - Poll until server is running (with timeout)
- `DestroyServer()` - Delete server/instance
- `UploadSSHKey()` - Upload SSH public key to provider

---

### Phase 2: Configuration Layer

#### 2.1 Add Config Struct

**File**: `pkg/config/config.go`

Add provider-specific config struct:

```go
type NewProviderConfig struct {
    InstanceID  string `json:"instance_id,omitempty"`  // Server ID
    IP          string `json:"ip"`
    SSHKey      string `json:"ssh_key"`
    SSHKeyName  string `json:"ssh_key_name,omitempty"`
    Username    string `json:"username"`
    Region      string `json:"region,omitempty"`
    Size        string `json:"size,omitempty"`         // Or "plan", "type" - provider-specific
    Provisioned bool   `json:"provisioned,omitempty"`
}

// Implement ProviderConfig interface
func (c *NewProviderConfig) GetIP() string         { return c.IP }
func (c *NewProviderConfig) GetUsername() string   { return c.Username }
func (c *NewProviderConfig) GetSSHKey() string     { return c.SSHKey }
func (c *NewProviderConfig) IsProvisioned() bool   { return c.Provisioned }
func (c *NewProviderConfig) GetServerID() string   { return c.InstanceID }
```

#### 2.2 Add Config Getter Method

```go
func (t *TargetConfig) GetNewProviderConfig() (*NewProviderConfig, error) {
    var config NewProviderConfig
    if err := t.GetProviderConfig("newprovider", &config); err != nil {
        return nil, err
    }
    return &config, nil
}
```

#### 2.3 Update GetSSHProviderConfig Switch

**File**: `pkg/config/config.go` (around line 226)

```go
func (t *TargetConfig) GetSSHProviderConfig() (ProviderConfig, error) {
    switch t.Provider {
    case "digitalocean":
        return t.GetDigitalOceanConfig()
    case "hetzner":
        return t.GetHetznerConfig()
    case "vultr":
        return t.GetVultrConfig()
    case "linode":
        return t.GetLinodeConfig()
    case "newprovider":  // ADD THIS
        return t.GetNewProviderConfig()
    case "flyio":
        return t.GetFlyioConfig()
    case "s3":
        return nil, fmt.Errorf("S3 is not an SSH-based provider")
    default:
        return nil, fmt.Errorf("unsupported provider: %s", t.Provider)
    }
}
```

---

### Phase 3: Orchestrator Integration

**File**: `pkg/deploy/orchestrator.go`

#### 3.1 Add Provider Import

**Line 18-19** (alphabetically with other providers):

```go
import (
    // ...
    _ "lightfold/pkg/providers/digitalocean"
    _ "lightfold/pkg/providers/hetzner"
    _ "lightfold/pkg/providers/linode"
    _ "lightfold/pkg/providers/newprovider"  // ADD THIS
    _ "lightfold/pkg/providers/vultr"
    // ...
)
```

**⚠️ Critical**: Without this import, the provider's `init()` function won't run, and the provider won't be registered!

#### 3.2 Add Case to getProvisioningParams()

**Around line 776**:

```go
func (o *Orchestrator) getProvisioningParams() (region, size, sshKeyPath, username, sshKeyName string, err error) {
    switch o.config.Provider {
    // ... existing cases ...
    case "newprovider":
        npConfig, e := o.config.GetNewProviderConfig()
        if e != nil {
            err = fmt.Errorf("failed to get New Provider config: %w", e)
            return
        }
        region = npConfig.Region
        size = npConfig.Size          // Or Plan, Type - match your config struct
        sshKeyPath = npConfig.SSHKey
        username = npConfig.Username
        sshKeyName = npConfig.SSHKeyName
    default:
        err = fmt.Errorf("unsupported provider for provisioning: %s", o.config.Provider)
    }
    return
}
```

#### 3.3 Add Case to updateProviderConfigWithServerInfo()

**Around line 835**:

```go
func (o *Orchestrator) updateProviderConfigWithServerInfo(server *providers.Server) error {
    switch o.config.Provider {
    // ... existing cases ...
    case "newprovider":
        npConfig, err := o.config.GetNewProviderConfig()
        if err != nil {
            return err
        }
        npConfig.IP = server.PublicIPv4
        npConfig.InstanceID = server.ID
        return o.config.SetProviderConfig("newprovider", npConfig)
    default:
        return fmt.Errorf("unsupported provider: %s", o.config.Provider)
    }
}
```

---

### Phase 4: Utility Integration

#### 4.1 Add to Provider Recovery (IP Recovery)

**File**: `cmd/utils/provider_recovery.go` (around line 88)

```go
func updateProviderConfigWithIP(target *config.TargetConfig, providerName, ip, serverID string) error {
    switch providerName {
    // ... existing cases ...
    case "newprovider":
        npConfig, err := target.GetNewProviderConfig()
        if err != nil {
            return err
        }
        npConfig.IP = ip
        npConfig.InstanceID = serverID
        return target.SetProviderConfig("newprovider", npConfig)
    default:
        return fmt.Errorf("unsupported provider: %s", providerName)
    }
}
```

#### 4.2 Add to Multi-App Server Setup

**File**: `cmd/utils/server_setup.go` (around line 100)

```go
func SetupTargetWithExistingServer(target *config.TargetConfig, serverIP string, userPort int) error {
    // ...
    switch serverState.Provider {
    // ... existing cases ...
    case "newprovider":
        npConfig := &config.NewProviderConfig{
            IP:          serverIP,
            InstanceID:  serverState.ServerID,
            SSHKey:      sshKey,
            Username:    "deploy",
            Provisioned: false,
        }
        target.SetProviderConfig("newprovider", npConfig)
    case "byos":
        // ...
    default:
        return fmt.Errorf("unsupported provider: %s", serverState.Provider)
    }
    // ...
}
```

---

### Phase 5: Command Layer Integration (Optional TUI)

If you want interactive provisioning flows with region/size selection:

#### 5.1 Add TUI Flow

**File**: `cmd/ui/sequential/provider_flows.go`

```go
func NewProviderProvisioningFlow(initialState map[string]string) []Step {
    return []Step{
        NewInputStep("newprovider_token", "New Provider API Token:", "", true),
        NewSelectStep(
            "newprovider_region",
            "Select Region:",
            func(state map[string]string) ([]string, []string, error) {
                // Fetch regions from API
                client := newprovider.NewClient(state["newprovider_token"])
                regions, err := client.ListRegions(context.Background())
                return regions.IDs, regions.Names, err
            },
        ),
        NewSelectStep(
            "newprovider_size",
            "Select Size:",
            func(state map[string]string) ([]string, []string, error) {
                // Fetch available sizes/plans
                client := newprovider.NewClient(state["newprovider_token"])
                sizes, err := client.ListSizes(context.Background())
                return sizes.IDs, sizes.Descriptions, err
            },
        ),
    }
}
```

#### 5.2 Register Flow in Create Command

**File**: `cmd/create.go`

Add case to switch statement for interactive flows.

---

## Integration Checklist

Use this checklist to ensure complete integration:

### Core Implementation
- [ ] Created `pkg/providers/newprovider/client.go`
- [ ] Implemented all `Provider` interface methods
- [ ] Added `init()` function with `providers.Register()`
- [ ] Tested provider methods independently

### Configuration Layer
- [ ] Added `NewProviderConfig` struct to `pkg/config/config.go`
- [ ] Implemented `ProviderConfig` interface methods
- [ ] Added `GetNewProviderConfig()` method
- [ ] Updated `GetSSHProviderConfig()` switch statement

### Orchestrator Layer
- [ ] Added provider import to `pkg/deploy/orchestrator.go`
- [ ] Added case to `getProvisioningParams()`
- [ ] Added case to `updateProviderConfigWithServerInfo()`

### Utility Layer
- [ ] Added case to `updateProviderConfigWithIP()` in `cmd/utils/provider_recovery.go`
- [ ] Added case to `SetupTargetWithExistingServer()` in `cmd/utils/server_setup.go`

### Command Layer (Optional)
- [ ] Added TUI flow to `cmd/ui/sequential/provider_flows.go`
- [ ] Registered flow in `cmd/create.go`

### Testing & Documentation
- [ ] Created provider tests in `pkg/providers/newprovider/client_test.go`
- [ ] Tested provisioning flow end-to-end
- [ ] Tested IP recovery scenario
- [ ] Tested multi-app deployment
- [ ] Updated `README.md` supported providers list
- [ ] Updated `AGENTS.md` provider architecture section
- [ ] Updated `TODO.md` to mark provider as completed

---

## Common Pitfalls

### 1. **Missing Provider Import** (Most Common!)

**Problem**: Provider works in isolation but fails with "unsupported provider for provisioning: newprovider"

**Cause**: Missing import in `pkg/deploy/orchestrator.go`

**Solution**: Add blank import:
```go
_ "lightfold/pkg/providers/newprovider"
```

### 2. **Inconsistent Provider Name**

**Problem**: Provider name in `Name()` doesn't match registration key

**Bad**:
```go
func init() {
    providers.Register("new-provider", ...)  // Hyphenated
}
func (c *Client) Name() string {
    return "newprovider"  // No hyphen - MISMATCH!
}
```

**Good**: Use consistent naming everywhere (prefer lowercase, no special chars)

### 3. **Missing Switch Cases**

**Problem**: Forgot to add provider to one of the switch statements

**Solution**: Use this checklist:
- `pkg/config/config.go` - `GetSSHProviderConfig()`
- `pkg/deploy/orchestrator.go` - `getProvisioningParams()`
- `pkg/deploy/orchestrator.go` - `updateProviderConfigWithServerInfo()`
- `cmd/utils/provider_recovery.go` - `updateProviderConfigWithIP()`
- `cmd/utils/server_setup.go` - `SetupTargetWithExistingServer()`

### 4. **Wrong Config Key**

**Problem**: Using wrong provider name when setting config

**Bad**:
```go
doConfig := &config.DigitalOceanConfig{...}
target.SetProviderConfig("newprovider", doConfig)  // WRONG!
```

**Good**:
```go
npConfig := &config.NewProviderConfig{...}
target.SetProviderConfig("newprovider", npConfig)  // Correct
```

### 5. **Not Handling SupportsSSH() Correctly**

**Problem**: VPS provider returns `false` or container platform returns `true`

**Solution**:
- VPS providers (Linode, DigitalOcean, Hetzner, Vultr): `return true`
- Container platforms (Fly.io): `return false`

### 6. **OAuth Permission Issues with SSH Key Endpoints**

**Problem**: Provider API returns 401/403 when uploading SSH keys due to strict OAuth scopes

**Linode Example**: Linode's SSH key management endpoints require specific OAuth permissions that may not be available to all API tokens.

**Solution**: If the provider supports passing raw SSH public keys during provisioning (like Linode's `AuthorizedKeys` field), bypass account-level key upload:

```go
func (c *Client) UploadSSHKey(ctx context.Context, name, publicKey string) (*providers.SSHKey, error) {
    // Return the raw public key as the "ID" - Provision() will use it directly
    return &providers.SSHKey{
        ID:        publicKey, // The raw key itself
        Name:      name,
        PublicKey: publicKey,
    }, nil
}

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
    var authorizedKeys []string
    for _, keyStr := range config.SSHKeys {
        // Check if this is a raw public key (starts with "ssh-")
        if strings.HasPrefix(keyStr, "ssh-") {
            authorizedKeys = append(authorizedKeys, keyStr)
        } else {
            // Fallback: fetch from API by ID
        }
    }

    // Pass raw keys directly to provisioning API
    createOpts := ProvisionOptions{
        AuthorizedKeys: authorizedKeys,
        // ...
    }
    // ...
}
```

This pattern avoids OAuth permission issues while maintaining full functionality.

### 7. **User Data Encoding Requirements**

**Problem**: Provider API returns 400 error: "user_data must be a base64 encoded string"

**Linode Example**: Linode requires cloud-init user_data to be base64 encoded, while other providers accept plain text.

**Solution**: Base64 encode the user_data before passing it to the provider's API:

```go
import "encoding/base64"

func (c *Client) Provision(ctx context.Context, config providers.ProvisionConfig) (*providers.Server, error) {
    // ...

    if config.UserData != "" {
        // Provider requires base64 encoding
        encodedUserData := base64.StdEncoding.EncodeToString([]byte(config.UserData))
        createOpts.Metadata = &ProviderMetadataOptions{
            UserData: encodedUserData,
        }
    }

    // ...
}
```

**Check your provider's API documentation** to see if they require base64 encoding for user_data/cloud-init data.

---

## Testing

### Unit Tests

Create `pkg/providers/newprovider/client_test.go`:

```go
package newprovider

import (
    "context"
    "testing"
)

func TestValidateCredentials(t *testing.T) {
    client := NewClient("fake-token")
    err := client.ValidateCredentials(context.Background())
    // Test with mock API responses
}

func TestProvision(t *testing.T) {
    // Test provisioning flow
}
```

### Manual End-to-End Test

```bash
# Build binary
make build

# Test provisioning
./lightfold deploy ~/test-project

# Expected flow:
# 1. Framework detection
# 2. Provider selection (select newprovider)
# 3. Token prompt
# 4. Region selection
# 5. Size selection
# 6. SSH key generation
# 7. Server provisioning
# 8. Configuration
# 9. Deployment

# Verify:
# - Server created on provider dashboard
# - Config saved to ~/.lightfold/config.json
# - State saved to ~/.lightfold/state/<target>.json
# - SSH works: ./lightfold ssh --target <target>

# Test IP recovery
rm ~/.lightfold/config.json
./lightfold sync --target <target>

# Should recover IP from provider API

# Test multi-app deployment
./lightfold deploy --server-ip <existing-ip> ~/another-project

# Should reuse existing server
```

---

## Examples

### Example 1: Linode Provider (VPS)

Linode integration follows this exact pattern:

**Files Modified**:
- `pkg/providers/linode/client.go` - Core implementation
- `pkg/config/config.go` - Added `LinodeConfig` struct
- `pkg/deploy/orchestrator.go` - Added import + 2 switch cases
- `cmd/utils/provider_recovery.go` - Added switch case
- `cmd/utils/server_setup.go` - Added switch case

**Commit**: See `26a87c3` for complete Linode implementation

### Example 2: Fly.io Provider (Container Platform)

Fly.io is unique because:
- `SupportsSSH()` returns `false`
- Uses `flyctl` for deployment (not SSH)
- Has custom deployer (`pkg/deploy/flyio_deployer.go`)

**Key Differences**:
- Skip SSH key upload
- Skip server wait (no active VMs)
- Use GraphQL API for app creation
- Use `flyctl` CLI for deployments

---

## Summary

Adding a new provider requires:

1. **Implement** `Provider` interface in `pkg/providers/newprovider/`
2. **Add** config struct to `pkg/config/config.go`
3. **Import** provider in `pkg/deploy/orchestrator.go` (critical!)
4. **Add switch cases** to 5 key functions (orchestrator + utilities)
5. **Test** provisioning, recovery, and multi-app flows
6. **Update** documentation

**Total LOC**: ~500-800 lines for a typical VPS provider

**Common Error**: Forgetting provider import causes "unsupported provider" error during provisioning

---

## Questions or Issues?

If you encounter problems:
1. Check the [Integration Checklist](#integration-checklist)
2. Review [Common Pitfalls](#common-pitfalls)
3. Compare your implementation to existing providers (Linode, DigitalOcean, Hetzner)
4. Open an issue on GitHub with details

---

*Last Updated*: 2025-10-19
*Contributors*: Lightfold maintainers
