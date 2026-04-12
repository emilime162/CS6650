# Album Store Terraform Infrastructure

This directory contains modular Terraform configuration to deploy the complete album-store infrastructure on AWS.

## Architecture

```
terraform/
├── main.tf              # Module orchestration
├── variables.tf         # Input variables
├── outputs.tf           # Output values
├── provider.tf          # AWS provider config
└── modules/
    ├── storage/         # DynamoDB tables + S3 bucket
    ├── iam/             # IAM role and policies
    ├── networking/      # Security groups
    └── compute/         # EC2 instance
```

## Prerequisites

1. **AWS CLI** configured with credentials
2. **Terraform** >= 1.0 installed
3. **SSH key pair** created in AWS (note the key name)
4. **S3 bucket name** chosen (must be globally unique)

## Quick Start

### 1. Configure Variables

Copy the example variables file and fill in your values:

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` and set:
- `s3_bucket_name` - unique name like `album-store-photos-yourname-12345`
- `key_name` - your AWS SSH key pair name (e.g., `my-key`)

### 2. Initialize Terraform

```bash
terraform init
```

This downloads the AWS provider and initializes modules.

### 3. Preview Changes

```bash
terraform plan
```

Review what will be created.

### 4. Deploy Infrastructure

```bash
terraform apply
```

Type `yes` to confirm. This creates:
- 2 DynamoDB tables (albums, photos)
- 1 S3 bucket with public read policy
- IAM role and instance profile
- Security group (ports 8080, 22)
- EC2 instance with user data script

**⏱️ Takes ~2-3 minutes**

### 5. Get Outputs

After deployment completes, view the outputs:

```bash
terraform output
```

Key outputs:
- `base_url` - Your API endpoint
- `instance_public_ip` - EC2 public IP
- `deployment_summary` - All important info

Example:
```
base_url = "http://3.80.123.45:8080"
instance_public_ip = "3.80.123.45"
```

### 6. Deploy Application

**If you used manual deployment mode** (default, `git_repo_url = ""`):

The EC2 instance is ready but waiting for the binary. Upload it:

```bash
# From the album-store root directory
make build-linux
make deploy HOST=ec2-user@<instance-ip> KEY=~/.ssh/your-key.pem
```

Or use SCP directly:
```bash
scp -i ~/.ssh/your-key.pem bin/album-store ec2-user@<instance-ip>:~/
ssh -i ~/.ssh/your-key.pem ec2-user@<instance-ip>
sudo systemctl start album-store
sudo systemctl status album-store
```

**If you used automated deployment** (`git_repo_url` set):

The service should already be running! Check status:
```bash
ssh -i ~/.ssh/your-key.pem ec2-user@<instance-ip> "sudo systemctl status album-store"
```

### 7. Test API

```bash
# Get the base_url from terraform output
BASE_URL=$(terraform output -raw base_url)

# Health check
curl $BASE_URL/health
# → {"status":"ok"}

# Create an album
curl -X PUT $BASE_URL/albums/test-album \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Album","description":"My first album"}'
```

### 8. Submit to ChaosArena

```bash
BASE_URL=$(terraform output -raw base_url)

curl -X POST https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/submit \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"you@northeastern.edu\",
    \"nickname\": \"your-nickname\",
    \"base_url\": \"$BASE_URL\",
    \"contract\": \"v1-album-store\"
  }"
```

## Deployment Modes

### Manual Deployment (Recommended for first setup)

**When to use:** Testing, development, or when you want control over when the app starts.

**How:** Leave `git_repo_url = ""` in `terraform.tfvars`

The user data script will:
- ✅ Install Go
- ✅ Configure systemd service
- ✅ Enable service
- ⏸️ Wait for you to upload the binary

Then you upload and start manually:
```bash
make deploy HOST=ec2-user@<ip> KEY=~/.ssh/key.pem
```

### Automated Deployment

**When to use:** CI/CD, production, or when your code is in a git repository.

**How:** Set `git_repo_url` and optionally `git_branch` in `terraform.tfvars`

```hcl
git_repo_url = "https://github.com/yourusername/album-store.git"
git_branch   = "main"
```

The user data script will:
- ✅ Install Go
- ✅ Clone repository
- ✅ Build binary
- ✅ Configure and start service
- ✅ Service running immediately

## Updating the Infrastructure

### Update Application Code

**Manual mode:**
```bash
make build-linux
make deploy HOST=ec2-user@<ip> KEY=~/.ssh/key.pem
```

**Automated mode:**
```bash
# Push changes to git, then:
terraform apply -replace="module.compute.aws_instance.album_store"
```

### Update Infrastructure

Modify variables in `terraform.tfvars`, then:
```bash
terraform plan   # Review changes
terraform apply  # Apply changes
```

### Update Instance Type

Edit `terraform.tfvars`:
```hcl
instance_type = "c5.2xlarge"  # For heavier load
```

Then:
```bash
terraform apply
```

## Destroying Infrastructure

⚠️ **This deletes everything!**

```bash
terraform destroy
```

Type `yes` to confirm. Resources deleted:
- EC2 instance
- Security group
- IAM role and instance profile
- DynamoDB tables (with all data!)
- S3 bucket (must be empty first)

**Important:** If S3 bucket has files, delete them first:
```bash
aws s3 rm s3://your-bucket-name --recursive
```

## Module Details

### Storage Module (`modules/storage/`)
- **DynamoDB tables:** `albums` (hash: album_id), `photos` (hash: album_id, range: photo_id)
- **S3 bucket:** Public read access for serving images
- **Billing:** PAY_PER_REQUEST for DynamoDB (no capacity planning)

### IAM Module (`modules/iam/`)
- **EC2 role:** Allows EC2 to assume role
- **Policies:** DynamoDB (PutItem, GetItem, etc.) + S3 (PutObject, GetObject, etc.)
- **Instance profile:** Attached to EC2 instance

### Networking Module (`modules/networking/`)
- **VPC:** Uses default VPC
- **Security group:** 
  - Inbound: 8080 (API), 22 (SSH)
  - Outbound: All traffic (for AWS API calls, package downloads)

### Compute Module (`modules/compute/`)
- **AMI:** Latest Amazon Linux 2023
- **User data:** Installs Go, optionally clones repo, sets up systemd service
- **Instance profile:** IAM role for AWS access
- **Root volume:** 20 GB GP3 (adjustable)

## Configuration Reference

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `s3_bucket_name` | Unique S3 bucket name | `album-store-photos-chen-98765` |
| `key_name` | AWS SSH key pair name | `my-aws-key` |

### Optional Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `aws_region` | `us-east-1` | AWS region |
| `environment` | `dev` | Environment tag |
| `albums_table_name` | `albums` | DynamoDB albums table |
| `photos_table_name` | `photos` | DynamoDB photos table |
| `instance_type` | `t3.xlarge` | EC2 instance type |
| `go_version` | `1.21.6` | Go version to install |
| `git_repo_url` | `""` | Git repo for auto-deploy |
| `git_branch` | `main` | Git branch to checkout |
| `allowed_cidr_blocks` | `["0.0.0.0/0"]` | Port 8080 access |
| `ssh_cidr_blocks` | `["0.0.0.0/0"]` | SSH access |

## Troubleshooting

### Service won't start

SSH into instance and check logs:
```bash
ssh -i ~/.ssh/key.pem ec2-user@<ip>
sudo journalctl -u album-store -n 50 -f
```

Check user data logs:
```bash
sudo cat /var/log/user-data.log
```

Check if binary exists:
```bash
ls -la /home/ec2-user/album-store
```

### Can't connect to API

1. Check security group allows port 8080
2. Verify service is running: `sudo systemctl status album-store`
3. Test locally on instance: `curl http://localhost:8080/health`

### S3 bucket name already taken

S3 bucket names must be globally unique. Try adding a timestamp:
```hcl
s3_bucket_name = "album-store-photos-chen-20260412"
```

### Terraform state is out of sync

If resources were modified outside Terraform:
```bash
terraform refresh  # Update state
terraform plan     # Check drift
```

## Cost Estimate

**Approximate monthly costs (us-east-1):**
- EC2 t3.xlarge: ~$120/month (on-demand)
- DynamoDB: Pay per request (~$1.25/million reads, $6.25/million writes)
- S3: ~$0.023/GB storage + ~$0.09/GB transfer out
- Data Transfer: First 100 GB/month free

**To minimize costs:**
- Stop EC2 when not in use: `aws ec2 stop-instances --instance-ids <id>`
- Use smaller instance type for testing: `t3.medium` (~$30/month)
- Destroy infrastructure when done: `terraform destroy`

## CI/CD Integration

Example GitHub Actions workflow snippet:

```yaml
- name: Deploy Infrastructure
  run: |
    cd terraform
    terraform init
    terraform apply -auto-approve \
      -var="s3_bucket_name=album-store-${{ github.sha }}" \
      -var="key_name=${{ secrets.AWS_KEY_NAME }}"
```

## Security Recommendations

For production:

1. **Restrict SSH access:**
   ```hcl
   ssh_cidr_blocks = ["YOUR.IP.ADDRESS/32"]
   ```

2. **Restrict API access** (if not public):
   ```hcl
   allowed_cidr_blocks = ["YOUR.OFFICE.CIDR/24"]
   ```

3. **Use Terraform backend** for state locking:
   ```hcl
   terraform {
     backend "s3" {
       bucket = "my-terraform-state"
       key    = "album-store/terraform.tfstate"
       region = "us-east-1"
     }
   }
   ```

4. **Enable CloudWatch logs:**
   Add to compute module for centralized logging.

5. **Use HTTPS:**
   Add ALB + ACM certificate for production.

## Additional Resources

- [Terraform AWS Provider Docs](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
- [Album Store Main README](../README.md)
- [Terraform Modules Best Practices](https://www.terraform.io/docs/language/modules/develop/index.html)
