# AWS EC2 Examples

This document provides real-world examples of deploying applications to AWS EC2 using Lightfold CLI.

## Table of Contents

1. [Deploy Next.js App to AWS EC2](#example-1-deploy-nextjs-app-to-aws-ec2)
2. [BYOS Mode with Existing EC2 Instance](#example-2-byos-mode-with-existing-ec2-instance)
3. [Multi-App Deployment on AWS](#example-3-multi-app-deployment-on-aws)
4. [Using AWS Profiles](#example-4-using-aws-profiles)
5. [Production Deployment with Elastic IP and Custom Domain](#example-5-production-deployment-with-elastic-ip-and-custom-domain)
6. [CI/CD Pipeline with AWS EC2](#example-6-cicd-pipeline-with-aws-ec2)

---

## Example 1: Deploy Next.js App to AWS EC2

Deploy a Next.js application to a fresh EC2 instance with auto-provisioning.

### Project Structure

```
my-nextjs-app/
├── app/
│   ├── page.tsx
│   └── layout.tsx
├── public/
├── package.json
├── next.config.js
└── tsconfig.json
```

### Step-by-Step Deployment

```bash
# Navigate to your Next.js project
cd my-nextjs-app

# Deploy to AWS EC2 (first time)
lightfold deploy --provider aws
```

### Interactive Prompts

```
🔍 Detected framework: Next.js (score: 8.5)

🔐 AWS Authentication
Enter AWS Access Key ID: AKIAIOSFODNN7EXAMPLE
Enter AWS Secret Access Key: ****************************************

✓ Credentials validated

🌍 Select AWS Region:
  > us-east-1 (N. Virginia)
    us-west-2 (Oregon)
    eu-west-1 (Ireland)
    ap-southeast-1 (Singapore)

📦 Select Instance Type:
    t3.micro (1 vCPU, 1GB RAM) - Free tier eligible
  > t3.small (2 vCPU, 2GB RAM) - $15/month
    t3.medium (2 vCPU, 4GB RAM) - $30/month
    m5.large (2 vCPU, 8GB RAM) - $70/month

🌐 Allocate Elastic IP? (Y/n)
ℹ Elastic IPs ensure your IP persists across instance stops/starts.
  - Cost: Free while running, ~$3.60/month if stopped
  - Without it: IP changes whenever instance is stopped/started
> Y

🔑 Generating SSH key pair...
✓ SSH key generated: ~/.lightfold/keys/lightfold_ed25519

🚀 Provisioning EC2 instance...
✓ Instance created: i-0123456789abcdef0
✓ Security group created: lightfold-my-nextjs-app
✓ Elastic IP allocated: 54.123.45.67
✓ Elastic IP associated

⚙️ Configuring server...
✓ Installed base packages
✓ Installed Node.js runtime
✓ Configured nginx
✓ Set up systemd service

📦 Deploying application...
✓ Uploaded code (Release: 20251024120000)
✓ Installed dependencies (npm install)
✓ Built application (npm run build)
✓ Started service
✓ Health check passed

🎉 Deployment successful!
URL: http://54.123.45.67
Target: my-nextjs-app
```

### Configuration Generated

**~/.lightfold/config.json:**
```json
{
  "targets": {
    "my-nextjs-app": {
      "project_path": "/Users/you/my-nextjs-app",
      "framework": "Next.js",
      "provider": "aws",
      "builder": "native",
      "provider_config": {
        "aws": {
          "instance_id": "i-0123456789abcdef0",
          "ip": "54.123.45.67",
          "ssh_key": "~/.lightfold/keys/lightfold_ed25519",
          "ssh_key_name": "lightfold-my-nextjs-app-20251024",
          "username": "ubuntu",
          "region": "us-east-1",
          "instance_type": "t3.small",
          "provisioned": true,
          "elastic_ip": "eipalloc-0123456789abcdef",
          "security_group_id": "sg-0123456789abcdef",
          "vpc_id": "vpc-0123456789abcdef",
          "subnet_id": "subnet-0123456789abcdef"
        }
      }
    }
  }
}
```

### Subsequent Deployments

```bash
# Make changes to your code
git add .
git commit -m "Update homepage"

# Redeploy (skips provisioning and configuration)
lightfold deploy

# Output:
# ✓ Target 'my-nextjs-app' already created
# ✓ Server already configured
# 📦 Deploying application...
# ✓ New commit detected: abc123...
# ✓ Deployed successfully
```

---

## Example 2: BYOS Mode with Existing EC2 Instance

Use Lightfold with an existing EC2 instance you've already created manually.

### Prerequisites

1. **Existing EC2 instance** (Ubuntu 22.04 recommended)
2. **SSH access** configured with key-based authentication
3. **Security group** with ports 22, 80, 443, and app ports (3000-9000) open
4. **Public IP address** of the instance

### Deployment Steps

```bash
cd my-django-app

lightfold deploy --provider byos \
  --ip 52.123.45.67 \
  --user ubuntu \
  --ssh-key ~/.ssh/my-aws-key.pem
```

### Interactive Flow

```
🔍 Detected framework: Django (score: 9.0)

🔌 BYOS Mode: Bring Your Own Server
✓ SSH connection verified: ubuntu@52.123.45.67

⚙️ Configuring server...
✓ Installed Python runtime
✓ Installed nginx
✓ Configured systemd service

📦 Deploying application...
✓ Uploaded code
✓ Installed dependencies (pip install -r requirements.txt)
✓ Ran migrations (python manage.py migrate)
✓ Collected static files
✓ Started service

🎉 Deployment successful!
URL: http://52.123.45.67
```

### Configuration

**~/.lightfold/config.json:**
```json
{
  "targets": {
    "my-django-app": {
      "project_path": "/Users/you/my-django-app",
      "framework": "Django",
      "provider": "byos",
      "provider_config": {
        "byos": {
          "ip": "52.123.45.67",
          "username": "ubuntu",
          "ssh_key": "~/.ssh/my-aws-key.pem",
          "provisioned": false
        }
      }
    }
  }
}
```

### Notes

- **No AWS API calls**: Lightfold only uses SSH
- **Manual cleanup**: When you destroy, Lightfold only removes local config (instance remains)
- **Security group**: Must be configured manually
- **Firewall**: Ensure UFW or iptables allows required ports

---

## Example 3: Multi-App Deployment on AWS

Deploy multiple applications to a single EC2 instance with automatic port allocation.

### Scenario

- **Instance**: Single `t3.medium` EC2 instance
- **Apps**:
  1. Frontend: Next.js (port 3000)
  2. API: FastAPI (port 3001)
  3. Admin: Django (port 3002)

### Step 1: Deploy First App (Next.js Frontend)

```bash
cd ~/projects/my-frontend
lightfold deploy --provider aws --target frontend-prod

# Provisions new EC2 instance
# IP: 54.123.45.67
# Port: 3000
```

### Step 2: Deploy Second App (FastAPI API)

```bash
cd ~/projects/my-api
lightfold deploy --target api-prod --server-ip 54.123.45.67

# Output:
# ✓ Detected existing server: 54.123.45.67
# ✓ Found 1 existing app: frontend-prod (port 3000)
# 🔌 Allocated port: 3001
# ⚙️ Configuring server...
# ✓ Python runtime already installed
# ✓ Configured nginx for api-prod
# 📦 Deploying application...
# ✓ Deployed successfully
# URL: http://54.123.45.67:3001
```

### Step 3: Deploy Third App (Django Admin)

```bash
cd ~/projects/my-admin
lightfold deploy --target admin-prod --server-ip 54.123.45.67

# Output:
# ✓ Detected existing server: 54.123.45.67
# ✓ Found 2 existing apps: frontend-prod (3000), api-prod (3001)
# 🔌 Allocated port: 3002
# ⚙️ Configuring server...
# ✓ Configured nginx for admin-prod
# 📦 Deploying application...
# ✓ Deployed successfully
# URL: http://54.123.45.67:3002
```

### View Server Status

```bash
lightfold server show 54.123.45.67
```

**Output:**
```
Server: 54.123.45.67
Provider: AWS EC2
Region: us-east-1
Instance Type: t3.medium (2 vCPU, 4GB RAM)
Instance ID: i-0123456789abcdef0

Deployed Apps (3):
┌─────────────────┬──────────────┬──────┬────────────┬─────────────────────┐
│ Target          │ Framework    │ Port │ Status     │ Last Deploy         │
├─────────────────┼──────────────┼──────┼────────────┼─────────────────────┤
│ frontend-prod   │ Next.js      │ 3000 │ ✓ Running  │ 2025-10-24 12:00:00 │
│ api-prod        │ FastAPI      │ 3001 │ ✓ Running  │ 2025-10-24 12:15:00 │
│ admin-prod      │ Django       │ 3002 │ ✓ Running  │ 2025-10-24 12:30:00 │
└─────────────────┴──────────────┴──────┴────────────┴─────────────────────┘

Installed Runtimes:
- Node.js (for Next.js)
- Python 3.11 (for FastAPI, Django)

Port Range: 3000-9000
Used Ports: 3000, 3001, 3002
Available Ports: 3003-9000
```

### Server State File

**~/.lightfold/servers/54.123.45.67.json:**
```json
{
  "ip": "54.123.45.67",
  "provider": "aws",
  "apps": [
    {
      "target_name": "frontend-prod",
      "framework": "Next.js",
      "port": 3000,
      "last_deploy": "2025-10-24T12:00:00Z"
    },
    {
      "target_name": "api-prod",
      "framework": "FastAPI",
      "port": 3001,
      "last_deploy": "2025-10-24T12:15:00Z"
    },
    {
      "target_name": "admin-prod",
      "framework": "Django",
      "port": 3002,
      "last_deploy": "2025-10-24T12:30:00Z"
    }
  ],
  "installed_runtimes": ["nodejs", "python"]
}
```

### Destroy One App (Automatic Runtime Cleanup)

```bash
lightfold destroy --target api-prod

# Output:
# ⚠️  Warning: This will destroy the following:
#    - Target: api-prod
#    - App on server: 54.123.45.67 (port 3001)
#    - Note: Server will remain running with 2 other apps
#
# Type 'api-prod' to confirm: api-prod
#
# ✓ Stopped service: api-prod
# ✓ Removed nginx config
# ✓ Cleaned up app directory
# ✓ Unregistered from server state
# ✓ Analyzing runtime dependencies...
#   - Node.js: Still needed by frontend-prod
#   - Python: Still needed by admin-prod
# ✓ Removed local config and state
# 🎉 Destroyed successfully
```

---

## Example 4: Using AWS Profiles

Use AWS CLI profiles for authentication instead of storing credentials in Lightfold.

### Step 1: Configure AWS Profile

```bash
aws configure --profile my-company
```

**~/.aws/credentials:**
```ini
[my-company]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
region = us-east-1
```

### Step 2: Set Environment Variable

```bash
export AWS_PROFILE=my-company
```

### Step 3: Deploy with Profile

```bash
cd my-app
lightfold deploy --provider aws
```

**Lightfold will automatically use the active AWS profile!**

### Step 4: Store Profile in Lightfold (Optional)

```bash
lightfold config set-token aws
# Enter: {"profile": "my-company"}
```

**~/.lightfold/tokens.json:**
```json
{
  "aws": "{\"profile\":\"my-company\"}"
}
```

Now you can deploy without setting `AWS_PROFILE` each time.

---

## Example 5: Production Deployment with Elastic IP and Custom Domain

Deploy a production app with Elastic IP, custom domain, and SSL.

### Step 1: Deploy with Elastic IP

```bash
cd my-production-app
lightfold deploy --provider aws --target prod

# Select:
# - Region: us-east-1
# - Instance: t3.medium
# - Elastic IP: Yes
```

### Step 2: Add Custom Domain

```bash
# Point your DNS to the Elastic IP
# A record: app.example.com -> 54.123.45.67

# Configure domain + SSL in Lightfold
lightfold domain add --domain app.example.com --target prod
```

**Interactive Flow:**
```
🌐 Adding domain to target: prod

✓ SSH connection verified
✓ Current IP: 54.123.45.67

Enable SSL with Let's Encrypt? (Y/n): Y

📜 Installing certbot...
✓ Certbot installed

🔐 Issuing SSL certificate...
✓ Certificate issued for app.example.com
✓ Auto-renewal enabled (certbot.timer)

⚙️ Updating nginx configuration...
✓ Configured HTTPS with automatic redirect
✓ Nginx reloaded

🎉 Domain configured successfully!
URL: https://app.example.com
```

### Step 3: Verify Deployment

```bash
curl -I https://app.example.com
```

**Output:**
```
HTTP/2 200
server: nginx
content-type: text/html
strict-transport-security: max-age=31536000; includeSubDomains
x-content-type-options: nosniff
x-frame-options: SAMEORIGIN
```

### Configuration

**~/.lightfold/config.json:**
```json
{
  "targets": {
    "prod": {
      "project_path": "/Users/you/my-production-app",
      "framework": "Next.js",
      "provider": "aws",
      "provider_config": {
        "aws": {
          "instance_id": "i-0123456789abcdef0",
          "ip": "54.123.45.67",
          "elastic_ip": "eipalloc-0123456789abcdef",
          "region": "us-east-1",
          "instance_type": "t3.medium",
          "provisioned": true
        }
      },
      "domain_config": {
        "domain": "app.example.com",
        "ssl_enabled": true,
        "ssl_manager": "certbot",
        "proxy_type": "nginx"
      }
    }
  }
}
```

---

## Example 6: CI/CD Pipeline with AWS EC2

Automate deployments with GitHub Actions.

### GitHub Actions Workflow

**.github/workflows/deploy.yml:**
```yaml
name: Deploy to AWS EC2

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Install Lightfold
        run: |
          curl -sSL https://github.com/theognis1002/lightfold-cli/releases/latest/download/lightfold-linux-amd64 -o /usr/local/bin/lightfold
          chmod +x /usr/local/bin/lightfold

      - name: Configure Lightfold
        run: |
          mkdir -p ~/.lightfold
          echo '${{ secrets.LIGHTFOLD_CONFIG }}' > ~/.lightfold/config.json
          echo '${{ secrets.LIGHTFOLD_TOKENS }}' > ~/.lightfold/tokens.json
          chmod 600 ~/.lightfold/tokens.json

      - name: Deploy to AWS EC2
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        run: |
          lightfold deploy --target production
```

### GitHub Secrets

Add these secrets to your repository:

1. **AWS_ACCESS_KEY_ID**: Your AWS access key
2. **AWS_SECRET_ACCESS_KEY**: Your AWS secret key
3. **LIGHTFOLD_CONFIG**: Contents of `~/.lightfold/config.json`
4. **LIGHTFOLD_TOKENS**: Contents of `~/.lightfold/tokens.json`

### First-Time Setup

```bash
# Deploy manually first to create config
lightfold deploy --provider aws --target production

# Copy config to GitHub secrets
cat ~/.lightfold/config.json | pbcopy  # macOS
cat ~/.lightfold/config.json | xclip -selection clipboard  # Linux

# Copy tokens to GitHub secrets
cat ~/.lightfold/tokens.json | pbcopy
```

### Deployment Process

1. Push code to `main` branch
2. GitHub Actions triggers workflow
3. Lightfold deploys to existing EC2 instance
4. Health checks verify deployment
5. Rollback on failure (optional)

---

## Additional Examples

### Example: Deploy Python Flask App

```bash
cd my-flask-app
lightfold deploy --provider aws --target flask-prod

# Lightfold auto-detects Flask
# Installs Python runtime
# Configures gunicorn + nginx
```

### Example: Deploy Rails App

```bash
cd my-rails-app
lightfold deploy --provider aws --target rails-prod

# Lightfold auto-detects Rails
# Installs Ruby runtime
# Configures Puma + nginx
# Runs database migrations
```

### Example: Deploy with Custom Instance Type

```bash
# For memory-intensive apps
lightfold create --provider aws --target heavy-app

# Select instance type: m5.xlarge (4 vCPU, 16GB RAM)
```

### Example: Check Deployment Status

```bash
# View all targets
lightfold status

# View specific target
lightfold status --target prod

# JSON output for automation
lightfold status --target prod --json
```

### Example: View Application Logs

```bash
# View logs
lightfold logs --target prod

# Stream logs in real-time
lightfold logs --target prod --tail

# Last 500 lines
lightfold logs --target prod --lines 500
```

### Example: Rollback Deployment

```bash
# Rollback to previous release
lightfold rollback --target prod

# Skip confirmation
lightfold rollback --target prod --force
```

### Example: SSH into Server

```bash
# Interactive SSH session
lightfold ssh --target prod

# Run one-off command
lightfold ssh --target prod --command "pm2 status"
```

---

## Best Practices

1. **Use Elastic IPs for production**: Avoid IP changes during maintenance
2. **Tag resources**: Lightfold auto-tags with `lightfold:target={name}`
3. **Enable auto-backups**: Use AWS EBS snapshots
4. **Monitor costs**: Use AWS Cost Explorer
5. **Rotate access keys**: Change keys every 90 days
6. **Use IAM policies with least privilege**: Only grant required permissions
7. **Enable CloudWatch monitoring**: Track instance metrics
8. **Set up billing alerts**: Avoid surprise charges
9. **Use custom domains with SSL**: Better security and UX
10. **Keep Lightfold updated**: `brew upgrade lightfold`

---

## Troubleshooting

### Issue: Deployment fails with "Connection refused"

**Solution**: Check security group rules

```bash
# Verify security group allows SSH (22), HTTP (80), HTTPS (443)
aws ec2 describe-security-groups --group-ids sg-0123456789abcdef
```

### Issue: App not accessible after deployment

**Solution**: Check nginx and app status

```bash
lightfold ssh --target prod --command "sudo systemctl status nginx"
lightfold ssh --target prod --command "sudo systemctl status lightfold-myapp"
lightfold logs --target prod
```

### Issue: Out of memory errors

**Solution**: Upgrade instance type

```bash
# Stop instance
aws ec2 stop-instances --instance-ids i-0123456789abcdef0

# Change instance type
aws ec2 modify-instance-attribute --instance-id i-0123456789abcdef0 --instance-type t3.medium

# Start instance
aws ec2 start-instances --instance-ids i-0123456789abcdef0

# Redeploy
lightfold sync --target prod
lightfold deploy --target prod
```

---

## Next Steps

- [Read AWS Setup Guide](./AWS_SETUP.md)
- [Configure custom domains](../README.md#domain--ssl-management)
- [Learn about multi-app deployments](../README.md#multi-app)
- [Explore all CLI commands](../README.md#commands)
