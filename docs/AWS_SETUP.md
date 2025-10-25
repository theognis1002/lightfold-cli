# AWS EC2 Setup Guide

This guide walks you through setting up AWS EC2 for use with Lightfold CLI.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Authentication Methods](#authentication-methods)
3. [Creating IAM User and Access Keys](#creating-iam-user-and-access-keys)
4. [Required IAM Permissions](#required-iam-permissions)
5. [AWS Profile Setup](#aws-profile-setup)
6. [First Deployment](#first-deployment)
7. [Elastic IP Considerations](#elastic-ip-considerations)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

- AWS account ([create one here](https://aws.amazon.com/free/))
- Lightfold CLI installed ([installation guide](../README.md#installation))
- Basic understanding of AWS IAM (Identity and Access Management)

---

## Authentication Methods

Lightfold supports two authentication methods for AWS:

### 1. Access Key ID + Secret Access Key (Direct Credentials)
- Recommended for: CI/CD pipelines, automation scripts
- Credentials stored securely in `~/.lightfold/tokens.json`

### 2. AWS Profile (~/.aws/credentials)
- Recommended for: Local development, multiple AWS accounts
- Uses existing AWS CLI profiles
- More secure (no credentials stored by Lightfold)

---

## Creating IAM User and Access Keys

### Step 1: Create IAM User

1. Go to [AWS IAM Console](https://console.aws.amazon.com/iam/)
2. Click **Users** → **Add users**
3. Enter username: `lightfold-deployer` (or your preferred name)
4. Select **Access key - Programmatic access**
5. Click **Next: Permissions**

### Step 2: Attach Permissions Policy

**Option A: Use Managed Policy (Quick Start)**

1. Click **Attach policies directly**
2. Search for and select `AmazonEC2FullAccess`
3. **Warning**: This grants full EC2 access. For production, use Option B below.

**Option B: Create Custom Policy (Recommended for Production)**

1. Click **Create policy**
2. Switch to the **JSON** tab
3. Paste the [IAM policy JSON](#iam-policy-json) from below
4. Click **Next: Tags** → **Next: Review**
5. Name the policy: `LightfoldDeploymentPolicy`
6. Click **Create policy**
7. Go back to the user creation flow and attach your new policy

### Step 3: Create Access Keys

1. After creating the user, click on the username
2. Go to **Security credentials** tab
3. Click **Create access key**
4. Select use case: **Command Line Interface (CLI)**
5. Check the confirmation box
6. Click **Next** → **Create access key**
7. **Important**: Download the CSV or copy the keys immediately (you won't see the secret key again!)

### Step 4: Store Credentials in Lightfold

**Method 1: During Deployment (Interactive)**
```bash
lightfold deploy --provider aws
# Lightfold will prompt you for your AWS Access Key ID and Secret Access Key
```

**Method 2: Manual Token Storage**
```bash
lightfold config set-token aws
# Enter your credentials when prompted
```

The credentials will be stored securely in `~/.lightfold/tokens.json` with restricted file permissions (0600).

---

## Required IAM Permissions

Lightfold requires the following IAM permissions to provision and manage EC2 instances:

### Core EC2 Permissions
- `ec2:RunInstances` - Launch new EC2 instances
- `ec2:DescribeInstances` - Fetch instance details and status
- `ec2:TerminateInstances` - Destroy instances during cleanup
- `ec2:DescribeRegions` - List available AWS regions
- `ec2:DescribeImages` - Lookup AMI IDs

### Networking Permissions
- `ec2:DescribeVpcs` - Find default VPC
- `ec2:DescribeSubnets` - Find default subnet
- `ec2:CreateSecurityGroup` - Create firewall rules
- `ec2:DeleteSecurityGroup` - Cleanup security groups
- `ec2:AuthorizeSecurityGroupIngress` - Configure inbound rules (HTTP/HTTPS/SSH)
- `ec2:DescribeSecurityGroups` - List security groups

### SSH Key Management
- `ec2:ImportKeyPair` - Upload SSH public keys
- `ec2:DeleteKeyPair` - Cleanup SSH keys (optional)
- `ec2:DescribeKeyPairs` - List SSH keys

### Elastic IP Permissions (Optional)
Required only if you choose to use Elastic IPs during provisioning:
- `ec2:AllocateAddress` - Allocate Elastic IP
- `ec2:AssociateAddress` - Attach Elastic IP to instance
- `ec2:DisassociateAddress` - Detach Elastic IP
- `ec2:ReleaseAddress` - Release Elastic IP on destroy
- `ec2:DescribeAddresses` - List Elastic IPs

### AMI Lookup Permissions
- `ssm:GetParameter` - Fetch latest Ubuntu 22.04 AMI ID via SSM Parameter Store

### Tagging Permissions
- `ec2:CreateTags` - Tag resources for tracking (instance, security group, Elastic IP)

---

## IAM Policy JSON

Copy this JSON to create a custom IAM policy with minimum required permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "LightfoldEC2Provisioning",
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances",
        "ec2:DescribeInstances",
        "ec2:TerminateInstances",
        "ec2:DescribeRegions",
        "ec2:DescribeImages",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightfoldNetworking",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeVpcs",
        "ec2:DescribeSubnets",
        "ec2:CreateSecurityGroup",
        "ec2:DeleteSecurityGroup",
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:DescribeSecurityGroups"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightfoldSSHKeys",
      "Effect": "Allow",
      "Action": [
        "ec2:ImportKeyPair",
        "ec2:DeleteKeyPair",
        "ec2:DescribeKeyPairs"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightfoldElasticIP",
      "Effect": "Allow",
      "Action": [
        "ec2:AllocateAddress",
        "ec2:AssociateAddress",
        "ec2:DisassociateAddress",
        "ec2:ReleaseAddress",
        "ec2:DescribeAddresses"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightfoldAMILookup",
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter"
      ],
      "Resource": "arn:aws:ssm:*::parameter/aws/service/canonical/ubuntu/server/22.04/stable/current/amd64/hvm/ebs-gp2/ami-id"
    }
  ]
}
```

**Explanation:**
- **LightfoldEC2Provisioning**: Core permissions for instance lifecycle
- **LightfoldNetworking**: VPC, subnet, and security group management
- **LightfoldSSHKeys**: SSH key pair management
- **LightfoldElasticIP**: Optional static IP allocation
- **LightfoldAMILookup**: Automatic Ubuntu AMI discovery

---

## AWS Profile Setup

If you prefer using AWS profiles (recommended for local development):

### Step 1: Install AWS CLI

```bash
# macOS
brew install awscli

# Linux
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# Windows
# Download from: https://awscli.amazonaws.com/AWSCLIV2.msi
```

### Step 2: Configure AWS Profile

```bash
aws configure --profile lightfold
```

You'll be prompted for:
- **AWS Access Key ID**: Enter your access key ID
- **AWS Secret Access Key**: Enter your secret access key
- **Default region name**: e.g., `us-east-1`
- **Default output format**: `json` (recommended)

This creates a profile in `~/.aws/credentials`:

```ini
[lightfold]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Step 3: Use Profile with Lightfold

```bash
# Set environment variable
export AWS_PROFILE=lightfold

# Deploy
lightfold deploy --provider aws
```

Or store the profile name in Lightfold:

```bash
lightfold config set-token aws
# Enter: {"profile": "lightfold"}
```

---

## First Deployment

### Step 1: Navigate to Your Project

```bash
cd your-app
```

### Step 2: Deploy with AWS EC2

```bash
lightfold deploy --provider aws
```

### Step 3: Interactive Prompts

Lightfold will guide you through:

1. **Authentication**: Enter AWS Access Key ID + Secret Key (or profile name)
2. **Region Selection**: Choose from available AWS regions (e.g., `us-east-1`, `eu-west-1`)
3. **Instance Type**: Select from curated list:
   - `t3.micro` (1 vCPU, 1GB RAM) - Free tier eligible
   - `t3.small` (2 vCPU, 2GB RAM) - Good for small apps
   - `t3.medium` (2 vCPU, 4GB RAM) - Good for medium apps
   - `m5.large` (2 vCPU, 8GB RAM) - Production workloads
4. **Elastic IP**: Choose whether to allocate a static IP (see [Elastic IP Considerations](#elastic-ip-considerations))
5. **SSH Key**: Auto-generated and uploaded to AWS

### Step 4: Wait for Deployment

Lightfold will:
1. ✓ Provision EC2 instance
2. ✓ Create security group with HTTP/HTTPS/SSH rules
3. ✓ Optionally allocate and associate Elastic IP
4. ✓ Configure server (nginx, runtime, systemd)
5. ✓ Deploy your application
6. ✓ Run health checks

### Step 5: Access Your App

```
🎉 Deployment successful!
URL: http://your-elastic-ip-or-instance-ip
```

---

## Elastic IP Considerations

### What is an Elastic IP?

An Elastic IP is a **static public IPv4 address** that persists across instance stops/starts. Without it, your instance gets a new IP every time it's stopped and started.

### Pros:
- ✓ **Persistent IP**: IP address never changes, even if instance is stopped
- ✓ **DNS Stability**: Point your domain's DNS to a fixed IP
- ✓ **Good for production**: Avoid DNS propagation delays

### Cons:
- ✗ **Cost**: ~$3.60/month if instance is stopped (free while associated with running instance)
- ✗ **AWS Quota**: Limited to 5 Elastic IPs per region (can request increase)

### When to Use Elastic IP?

**Use Elastic IP if:**
- You plan to use a custom domain
- You may stop/start the instance frequently
- You need a stable IP for DNS or external integrations

**Skip Elastic IP if:**
- Testing or development environment
- Instance will always be running (no stops)
- Using Lightfold's dynamic IP recovery is acceptable

### Pricing

| Scenario | Cost |
|----------|------|
| Elastic IP associated with running instance | **Free** |
| Elastic IP associated with stopped instance | **~$3.60/month** |
| Ephemeral IP (default) | **Free** |

### Example Prompt

```
Allocate Elastic IP? (Y/n)
ℹ Elastic IPs ensure your IP persists across instance stops/starts.
  - Cost: Free while running, ~$3.60/month if stopped
  - Without it: IP changes whenever instance is stopped/started
```

---

## Troubleshooting

### Issue: "No default VPC found"

**Solution**: Create a default VPC in your AWS region

1. Go to [VPC Console](https://console.aws.amazon.com/vpc/)
2. Click **Actions** → **Create Default VPC**
3. Retry deployment

### Issue: "UnauthorizedOperation" errors

**Cause**: IAM user lacks required permissions

**Solution**: Attach the [IAM policy JSON](#iam-policy-json) to your IAM user

### Issue: "InvalidKeyPair.NotFound"

**Cause**: SSH key pair not found in AWS

**Solution**: Lightfold auto-generates SSH keys. If you see this error:

```bash
# Re-run with fresh SSH key generation
lightfold create --target myapp --provider aws
```

### Issue: "InvalidGroup.InUse" when destroying

**Cause**: Security group still attached to terminated instance

**Solution**: Lightfold waits for instance termination before deleting security group. If the error persists:

1. Check [EC2 Console](https://console.aws.amazon.com/ec2/) for terminated instances
2. Wait a few minutes for AWS to fully detach resources
3. Retry destroy:
   ```bash
   lightfold destroy --target myapp
   ```

### Issue: Elastic IP not released after destroy

**Cause**: Destroy may have failed partway through cleanup

**Solution**: Manually release Elastic IP

1. Go to [EC2 Console → Elastic IPs](https://console.aws.amazon.com/ec2/v2/home#Addresses:)
2. Select the Elastic IP
3. Click **Actions** → **Release Elastic IP address**
4. Confirm

### Issue: "InvalidAMI: The image id '[ami-xxxxx]' does not exist"

**Cause**: AMI not available in selected region

**Solution**: Lightfold uses SSM Parameter Store to auto-detect Ubuntu 22.04 AMI. If this fails:

1. Check AWS SSM Parameter Store in your region
2. Report issue to Lightfold with region name
3. Temporary workaround: Use a different region

### Issue: AWS_PROFILE environment variable ignored

**Cause**: Lightfold may have stored credentials in tokens.json

**Solution**: Remove stored credentials

```bash
# Check stored tokens
cat ~/.lightfold/tokens.json

# Remove AWS token
lightfold config remove-token aws  # (if this command exists)

# Or manually edit
nano ~/.lightfold/tokens.json
# Remove "aws": "..." entry
```

Then set `AWS_PROFILE` and re-run deployment.

### Issue: "Service Unavailable" or timeouts

**Cause**: AWS API rate limiting or service issues

**Solution**: Wait a few minutes and retry. AWS may be experiencing outages or rate limits.

---

## Additional Resources

- [AWS EC2 Pricing](https://aws.amazon.com/ec2/pricing/)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [AWS Free Tier](https://aws.amazon.com/free/)
- [Lightfold Documentation](https://lightfold.mintlify.app/)
- [Lightfold GitHub Issues](https://github.com/theognis1002/lightfold-cli/issues)

---

## Security Best Practices

1. **Use IAM Policies with Least Privilege**: Only grant permissions Lightfold needs
2. **Rotate Access Keys Regularly**: Change keys every 90 days
3. **Enable MFA on IAM User**: Add extra layer of security
4. **Use AWS Profiles for Local Development**: Avoid storing credentials in Lightfold
5. **Monitor CloudTrail Logs**: Track all API calls made by Lightfold
6. **Tag All Resources**: Lightfold auto-tags with `lightfold:target={target-name}`
7. **Delete Unused Resources**: Run `lightfold destroy` when done testing

---

## Next Steps

- [Deploy your first app](../README.md#quick-start)
- [Learn about multi-app deployments](../README.md#multi-app)
- [Configure custom domains](../README.md#domain--ssl-management)
- [Explore advanced CLI commands](../README.md#commands)
