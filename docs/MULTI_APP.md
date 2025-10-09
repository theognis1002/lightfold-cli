# Multi-App Deployment - Technical Documentation

## Overview

This document provides comprehensive documentation for Lightfold's multi-app deployment feature, which enables deploying multiple applications to a single VPS/VM using automatic reverse proxy management (Caddy preferred, with Nginx fallback). Users can run multiple apps on one server with automatic subdomain routing, SSL certificate management, and port allocation.

## Goals

- **Seamless UX**: Zero manual configuration - CLI handles everything
- **Automatic SSL**: Caddy's native ACME support eliminates need for certbot
- **Cost Efficiency**: Run multiple apps on one VPS to reduce hosting costs
- **DRY Architecture**: Reuse existing registry patterns (proxy, SSL managers)
- **Safe Port Allocation**: Automatic port management prevents conflicts
- **Idempotent**: Multiple deployments to same server are safe

## Design Philosophy

**Opinionated Choices:**
1. **Caddy over Nginx** for multi-app: Automatic SSL, simpler config, zero-downtime reloads
2. **Subdomain-based routing**: Each app gets `app-name.domain.com` by default
3. **Automatic port allocation**: Sequential ports starting from 3000
4. **Shared server state**: Track all apps deployed to a server in centralized state
5. **Graceful degradation**: Falls back to Nginx if Caddy unavailable

---

## Implementation Status

### Completed Phases

#### Phase 1: Server State Infrastructure ✅

**Files Created:**
- `pkg/state/server.go` - Server state tracking
- `pkg/state/ports.go` - Port allocation logic

**State Management:**

```go
type ServerState struct {
    ServerIP     string        // Server IP address
    Provider     string        // Cloud provider name
    ServerID     string        // Provider's server ID
    ProxyType    string        // "nginx" or "caddy"
    RootDomain   string        // Optional root domain
    DeployedApps []DeployedApp // All apps on this server
    NextPort     int           // Next available port
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type DeployedApp struct {
    TargetName string    // Lightfold target name
    AppName    string    // App identifier
    Port       int       // Assigned port
    Domain     string    // Full domain (or empty)
    Framework  string    // Detected framework
    LastDeploy time.Time
}
```

**Storage:** `~/.lightfold/servers/<server-ip>.json`

**Port Allocation:**
- **Port Range:** 3000-9000
- **Allocation Strategy:** Sequential with gap filling
- **Conflict Detection:** Automatic validation
- **Statistics:** Real-time usage tracking

**Functions:**
- `AllocatePort(serverIP)` - Allocates next available port
- `GetAppPort(serverIP, targetName)` - Gets app's assigned port
- `IsPortAvailable(serverIP, port)` - Checks port availability
- `ReleasePort(serverIP, port)` - Frees a port
- `GetPortStatistics(serverIP)` - Returns usage stats
- `DetectPortConflicts(serverIP)` - Finds conflicts

#### Phase 4: CLI Integration (Partial) ✅

**File:** `cmd/common.go`

Added helper functions:

1. **`getOrAllocatePort(target, targetName)`**
   - Intelligently allocates ports for applications
   - Checks target config first
   - Uses server state for automatic port allocation
   - Falls back to framework detection

2. **`registerAppWithServer(target, targetName, port, framework)`**
   - Registers applications in server state
   - Tracks deployment metadata
   - Associates domains with apps

3. **`updateServerStateFromTarget(target, targetName)`**
   - Initializes server state from target configuration
   - Synchronizes provider metadata
   - Sets up proxy and domain configuration

#### Phase 5: Server Commands ✅

**File:** `cmd/server.go`

Created new `lightfold server` command with subcommands:

**1. `lightfold server list`**
- Lists all servers tracked by Lightfold
- Shows deployed applications per server
- Displays port usage statistics
- Detects and reports port conflicts

**2. `lightfold server show <server-ip>`**
- Detailed view of a specific server
- Configuration details (provider, proxy, domains)
- Port allocation statistics and next available port
- All deployed applications with full details
- Associated targets from config
- Port conflict detection

#### Phase 6: Enhanced Existing Commands ✅

**1. Enhanced `lightfold status`** (`cmd/status.go`)
- Shows when an app shares a server with other applications
- Lists other apps deployed to the same server
- Displays the app's allocated port

**2. Enhanced `lightfold destroy`** (`cmd/destroy.go`)
- Unregisters app from server state when destroyed
- Shows notification if other apps remain on the server
- Automatically cleans up server state when last app is removed

### Remaining Work

#### Phase 2: Caddy Proxy Manager (Not Started)

**Files to Create:**
- `pkg/proxy/caddy/manager.go` - Caddy proxy implementation
- `pkg/proxy/caddy/templates.go` - Caddyfile generation
- `pkg/ssl/caddy/manager.go` - Caddy SSL wrapper (no-op)

**Tests to Add:**
- `test/proxy/caddy_test.go` - Caddy config generation
- `test/proxy/caddy_integration_test.go` - End-to-end Caddy setup

#### Phase 3: Enhanced Nginx Manager (Not Started)

**Files to Modify:**
- `pkg/proxy/nginx/manager.go` - Add ConfigureMultiApp(), fix server_name generation
- Update templates to use specific domains, not "_"

#### Phase 4: Remaining CLI Integration Tasks

**1. Server State Integration in Deployment Flow**
- Update deploy/push command to call `registerAppWithServer()`
- Update deploy/push to use `getOrAllocatePort()` for port assignment
- Ensure `target.ServerIP` and `target.Port` are saved to config after deployment

**2. Domain Configuration Updates for Multi-App**
- Update `configureDomainAndSSL()` to use allocated ports
- Ensure proxy configuration uses correct port for each app
- Update server state with domain information
- Handle multiple apps with different domains on same server

**3. Deploy Command Updates for Server Selection**
- Add `--server-ip` flag to deploy command
- Allow users to specify existing server for deployment
- Implement logic to detect if server is already configured
- Skip server provisioning if deploying to existing server

---

## Architecture

### State File Organization

```
~/.lightfold/
├── config.json              # All target configurations
├── state/
│   ├── api-app.json         # Target-specific state
│   ├── web-app.json
│   └── admin-app.json
└── servers/
    ├── 192.168.1.100.json   # Server state
    └── 192.168.1.101.json
```

### Separation of Concerns

1. **Target Config** (`config.Config`): User-facing deployment configuration
2. **Target State** (`state.TargetState`): Per-target deployment state
3. **Server State** (`state.ServerState`): Server-level multi-app state

This separation allows:
- Target-specific operations to remain independent
- Server-wide operations to manage resources holistically
- Clean migration path from single-app to multi-app setups

### Port Allocation Strategy

The port allocation algorithm:
1. Starts at port 3000
2. Finds next available port sequentially
3. Wraps around to reuse freed ports
4. Detects exhaustion (all 6000 ports used)
5. Handles conflicts gracefully

---

## Proposed Future Architecture

### Caddy Proxy Manager

Caddy is preferred for multi-app deployments because:
- **Automatic HTTPS**: Native ACME integration (Let's Encrypt)
- **Zero-downtime reloads**: JSON API for live config updates
- **Simpler config**: No separate SSL manager needed
- **Better multi-domain**: Built for multiple sites out of the box

**Caddyfile Template (Multi-App):**
```caddyfile
# App 1: myapp.example.com -> :3000
myapp.example.com {
    reverse_proxy localhost:3000
    encode gzip
    log {
        output file /var/log/caddy/myapp.log
    }
}

# App 2: api.example.com -> :3001
api.example.com {
    reverse_proxy localhost:3001
    encode gzip
    log {
        output file /var/log/caddy/api.log
    }
}
```

### Enhanced Nginx Manager

**Updated Nginx Template (Multi-App):**
```nginx
# App-specific server_name (not catch-all "_")
server {
  listen 80;
  server_name myapp.example.com;  # Specific domain

  location / {
    proxy_pass http://127.0.0.1:3000;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

---

## User Experience

### Current Commands

**`lightfold server list`**
```bash
$ lightfold server list
Servers (2):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

192.168.1.100
  Provider:   digitalocean
  Proxy:      nginx
  Domain:     example.com
  Apps:       3 deployed

  Deployed Applications:
    • api-app (Express.js) - Port 3000 - api.example.com
      Last deployed: 2025-01-15 14:30
    • web-app (Next.js) - Port 3001 - example.com
      Last deployed: 2025-01-15 15:45
    • admin-app (Django) - Port 5000 - admin.example.com
      Last deployed: 2025-01-14 10:20

  Port Usage: 3 used, 6997 available
```

**`lightfold server show <ip>`**
```bash
$ lightfold server show 192.168.1.100
Server: 192.168.1.100
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Configuration:
  Provider:    digitalocean
  Server ID:   123456789
  Proxy:       nginx
  Root Domain: example.com
  Created:     2025-01-10 09:00:00
  Updated:     2025-01-15 15:45:12

Port Allocation:
  Range:       3000 - 9000
  Used:        3
  Available:   6997
  Next Port:   3002

Deployed Applications (3):
  1. api-app
     Framework: Express.js
     Port:      3000
     Domain:    api.example.com
     Deployed:  2025-01-15 14:30 (1 hour ago)

  2. web-app
     Framework: Next.js
     Port:      3001
     Domain:    example.com
     Deployed:  2025-01-15 15:45 (just now)

  3. admin-app
     Framework: Django
     Port:      5000
     Domain:    admin.example.com
     Deployed:  2025-01-14 10:20 (1 day ago)
```

### Planned User Flows

#### Flow 1: First App on New Server

```bash
$ cd my-nextjs-app
$ lightfold deploy

Provider? [digitalocean, vultr, hetzner, byos]: digitalocean
Region? [nyc1]: nyc1
Size? [s-1vcpu-1gb]: s-1vcpu-1gb

# Server provisioned: 192.168.1.100

Add custom domain? (Y/n): y
Domain: example.com

# Since this is the first app on this server:
Detected proxy: Caddy (with automatic SSL) ✓
Subdomain for this app? [my-nextjs-app]: (press enter)
Full domain: my-nextjs-app.example.com

Creating server... ✓
Installing Caddy... ✓
Allocating port: 3000 ✓
Deploying app... ✓
Configuring domain: my-nextjs-app.example.com ✓
SSL certificate issued automatically ✓

Your app is live at: https://my-nextjs-app.example.com

DNS Configuration Required:
┌────────────────────────────────────────┐
│ Type:  A                               │
│ Name:  *                               │
│ Value: 192.168.1.100                   │
│ TTL:   3600                            │
└────────────────────────────────────────┘

Note: Use wildcard (*) to support all subdomains
```

#### Flow 2: Second App on Existing Server

```bash
$ cd my-django-api
$ lightfold deploy

Provider? [digitalocean, vultr, hetzner, byos]: digitalocean

# NEW PROMPT: Detect existing servers from same provider
Found existing server: 192.168.1.100 (1 app deployed)
Deploy to existing server? (Y/n): y

Add custom domain? (Y/n): y
Root domain: example.com
Subdomain for this app? [my-django-api]: api

Connecting to server 192.168.1.100... ✓
Allocating port: 3001 ✓ (automatically assigned)
Deploying app... ✓
Updating Caddy config... ✓
Adding domain: api.example.com ✓
SSL certificate issued automatically ✓

Your app is live at: https://api.example.com

Server Status:
┌──────────────────────────────────────────┐
│ Server: 192.168.1.100                    │
│ Apps:   2                                │
│   - my-nextjs-app.example.com (port 3000)│
│   - api.example.com (port 3001)          │
└──────────────────────────────────────────┘
```

---

## Edge Cases & Solutions

### 1. Port Conflicts
**Problem:** Two apps assigned same port due to race condition or manual intervention.

**Solution:**
- Always query server state before allocation
- Include port check in deployment health checks
- `lightfold server show` detects conflicts

### 2. DNS Wildcard Requirements
**Problem:** User doesn't set up wildcard DNS for multi-subdomain scenario.

**Solution:**
- Detect if multiple apps share root domain
- Show clear DNS instructions: "Set up wildcard (* A record)"
- Provide DNS validation before deployment

### 3. Mixed Proxy Types on Same Server
**Problem:** First app uses Nginx, user tries to deploy second app and system wants Caddy.

**Solution:**
- Server state tracks proxy type
- Always use same proxy as first app on server
- Future: Provide migration command

### 4. Server IP Changes (BYOS Rebuilt)
**Problem:** Server IP changes but local state references old IP.

**Solution:**
- Extend `lightfold sync` to update server state file when IP changes
- Update all target configs pointing to old IP

### 5. Concurrent Deployments to Same Server
**Problem:** Two deploys to same server modify Caddy/Nginx config simultaneously.

**Solution:**
- Implement file locking on server: `/tmp/lightfold-deploy.lock`
- CLI waits up to 60s for lock release
- Show: "Another deployment in progress, waiting..."

---

## Testing Strategy

### Unit Tests
- Port allocation logic (conflicts, exhaustion, gaps) ✅
- Server state CRUD operations ✅
- Caddy/Nginx template generation (pending)
- Config serialization/deserialization ✅

### Integration Tests (Pending)
- Multi-app deployment to same server (2-3 apps)
- Mixed provider scenarios (DO + BYOS)
- Port conflict resolution
- Proxy config reloads

### Manual Testing Scenarios (Pending)
1. Deploy 3 Next.js apps to one DigitalOcean droplet
2. Add Django app to existing server via BYOS
3. Destroy middle app, deploy new app (port reuse)
4. Migrate server from Nginx to Caddy

---

## Performance Considerations

### Deployment Speed
- **First app on server:** +30s (Caddy installation)
- **Subsequent apps:** +5s (Caddy config reload via API)
- **Nginx alternative:** +10s per app (separate certbot calls)

### Resource Usage
- **Caddy:** ~10-15 MB RAM per process (single process handles all apps)
- **Nginx:** ~5-10 MB RAM + 10 MB per app (certbot timers)
- **Port overhead:** None (just metadata tracking)

### Scalability Limits
- **Apps per server:** 1000+ (limited by port range 3000-9000)
- **Config reload time:** O(1) for Caddy (API), O(n) for Nginx
- **Recommended:** 10-20 apps per $6/month droplet (based on traffic)

---

## Security Considerations

### SSL Certificate Management
- **Caddy:** Automatic ACME, renewal handled internally
- **Let's Encrypt rate limits:** 50 certs/week tracked automatically
- **Private keys:** Caddy stores in `/var/lib/caddy`

### Port Isolation
- All apps bind to `127.0.0.1` (localhost only)
- Only Caddy/Nginx exposed to internet (ports 80/443)
- No direct app-to-app communication (firewall rules unchanged)

### State File Security
- Server state files: `~/.lightfold/servers/*.json` (0644 permissions)
- No secrets stored in server state
- API tokens remain in `~/.lightfold/tokens.json` (0600)

---

## Benefits

1. **Resource Efficiency:** Deploy multiple apps to one server
2. **Cost Savings:** Reduce number of VMs needed
3. **Easy Management:** Server-centric view of deployments
4. **Automatic Port Management:** No manual port configuration
5. **Conflict Prevention:** Built-in detection and validation
6. **Clean Separation:** Target and server concerns are independent
7. **Migration Path:** Existing single-app deployments work unchanged

---

## Future Enhancements (Post-V1)

1. **Load Balancing:** Multiple instances of same app on one server
2. **Container Support:** Option to deploy apps as Docker containers
3. **Database Sharing:** Shared PostgreSQL/Redis across apps
4. **Resource Limits:** CPU/memory quotas per app
5. **App Grouping:** Logical groups of related apps
6. **Monitoring Dashboard:** Web UI to view all apps across servers
7. **Auto-scaling:** Horizontal scaling across multiple servers
8. **Blue/Green at Server Level:** Deploy entire server configs atomically

---

**Document Status:** In Progress - Phases 1, 4-6 Complete
**Last Updated:** 2025-10-09
**Next Steps:** Complete Phase 2 (Caddy), Phase 3 (Enhanced Nginx), Remaining Phase 4 tasks
