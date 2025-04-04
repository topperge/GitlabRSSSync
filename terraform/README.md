# Terraform Configuration for GitlabRSSSync S3 Backup

This directory contains Terraform configurations to set up AWS infrastructure for GitlabRSSSync's SQLite backup feature. The resources created include:

1. An S3 bucket for storing SQLite database backups
2. IAM role with restricted access to the S3 bucket
3. IAM policy limiting permissions to only what's needed for backups
4. Instance profile for EC2 deployments

## Prerequisites

- [Terraform](https://www.terraform.io/downloads.html) v1.2.0+
- AWS account with permissions to create S3 buckets and IAM resources
- AWS CLI configured with appropriate credentials

## Usage

### Basic Setup

1. Initialize the Terraform working directory:

```bash
terraform init
```

2. Review the planned changes:

```bash
terraform plan
```

3. Apply the configuration:

```bash
terraform apply
```

4. When finished, you can destroy all resources:

```bash
terraform destroy
```

### Customization

You can customize the deployment by changing variables in several ways:

- Edit `variables.tf` default values directly
- Create a `terraform.tfvars` file with your custom values
- Pass variables via command line:

```bash
terraform apply -var="environment=prod" -var="aws_region=us-west-2"
```

### Important Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `aws_region` | AWS region for all resources | `us-east-1` |
| `environment` | Environment name (dev, test, staging, prod) | `dev` |
| `bucket_prefix` | Prefix for S3 bucket name | `gitlabrsssync-backup` |
| `resource_prefix` | Prefix for IAM resources | `gitlabrsssync` |

## Integration with GitlabRSSSync

After applying this Terraform configuration, you'll receive outputs that can be used to configure GitlabRSSSync:

### For EC2 Deployments

1. Launch EC2 instances with the created instance profile
2. Configure GitlabRSSSync with these environment variables:
   ```
   USE_SQLITE=true
   DB_PATH=/path/to/your/database.db
   S3_ENABLED=true
   S3_REGION=<s3_bucket_region from output>
   S3_BUCKET_NAME=<s3_bucket_name from output>
   S3_KEY_PREFIX=gitlabrsssync
   ```

### For EKS Deployments with IRSA

1. Annotate your deployment or pod with:
   ```yaml
   annotations:
     eks.amazonaws.com/role-arn: <iam_role_arn from output>
   ```

2. Configure GitlabRSSSync with the same environment variables as above

### For Other Deployment Methods

If you're not using EC2 with instance profiles or EKS with IRSA, you'll need to create an IAM user with programmatic access and attach the created policy to it. Then use the access and secret keys in your GitlabRSSSync configuration.

## Security Considerations

- The S3 bucket enforces server-side encryption
- Public access is blocked by default
- All access requires TLS
- The IAM policy follows the principle of least privilege
- Lifecycle rules automatically manage object retention and transitions

## Customizing Lifecycle Rules

The default lifecycle configuration:
- Transitions objects to STANDARD_IA after 30 days
- Transitions objects to GLACIER after 90 days
- Expires objects after 365 days
- Expires noncurrent versions after 30 days

Adjust these settings via variables or by modifying `s3.tf`.
