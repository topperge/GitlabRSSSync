# Terraform outputs that provide useful information about the created resources

output "s3_bucket_name" {
  description = "The name of the S3 bucket created for GitlabRSSSync backups"
  value       = aws_s3_bucket.backup.bucket
}

output "s3_bucket_arn" {
  description = "The ARN of the S3 bucket"
  value       = aws_s3_bucket.backup.arn
}

output "s3_bucket_region" {
  description = "The region where the S3 bucket is located"
  value       = aws_s3_bucket.backup.region
}

output "s3_bucket_domain_name" {
  description = "The domain name of the bucket"
  value       = aws_s3_bucket.backup.bucket_domain_name
}

output "iam_role_name" {
  description = "The name of the IAM role created for GitlabRSSSync"
  value       = aws_iam_role.gitlabrsssync.name
}

output "iam_role_arn" {
  description = "The ARN of the IAM role created for GitlabRSSSync"
  value       = aws_iam_role.gitlabrsssync.arn
}

output "iam_policy_arn" {
  description = "The ARN of the IAM policy for S3 access"
  value       = aws_iam_policy.gitlabrsssync_s3.arn
}

output "instance_profile_name" {
  description = "The name of the instance profile for EC2 instances"
  value       = aws_iam_instance_profile.gitlabrsssync.name
}

output "instance_profile_arn" {
  description = "The ARN of the instance profile for EC2 instances"
  value       = aws_iam_instance_profile.gitlabrsssync.arn
}

output "environment_variables" {
  description = "Environment variables to configure GitlabRSSSync for S3 backup"
  value = {
    S3_ENABLED     = "true"
    S3_REGION      = var.aws_region
    S3_BUCKET_NAME = aws_s3_bucket.backup.bucket
    S3_KEY_PREFIX  = "gitlabrsssync"
    # Access keys not included - use IAM role instead for better security
  }
}

output "eks_pod_annotation" {
  description = "Annotation for EKS pods to use the IAM role (when using IRSA)"
  value = {
    "eks.amazonaws.com/role-arn" = aws_iam_role.gitlabrsssync.arn
  }
}
