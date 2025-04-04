# IAM Role for GitlabRSSSync
resource "aws_iam_role" "gitlabrsssync" {
  name = "${var.resource_prefix}-gitlabrsssync-role-${var.environment}"
  tags = local.tags

  # Trust relationship policy document
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"  # For EC2 instances
        }
        Action = "sts:AssumeRole"
      },
      {
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"  # For EKS pods with IRSA
        }
        Action = "sts:AssumeRole"
      }
    ]
  })

  # Optional: Add condition for OIDC provider if using EKS with IRSA
  # depends_on = [aws_iam_openid_connect_provider.eks]
}

# IAM Policy document for S3 access
data "aws_iam_policy_document" "gitlabrsssync_s3" {
  statement {
    sid    = "AllowS3BackupOperations"
    effect = "Allow"
    actions = [
      "s3:ListBucket",        # List bucket contents
      "s3:GetObject",         # Retrieve backups
      "s3:PutObject",         # Create backups
      "s3:DeleteObject",      # Delete old backups
      "s3:GetBucketLocation"  # Get bucket region
    ]
    resources = [
      aws_s3_bucket.backup.arn,
      "${aws_s3_bucket.backup.arn}/*"
    ]
  }
}

# IAM Policy for GitlabRSSSync
resource "aws_iam_policy" "gitlabrsssync_s3" {
  name        = "${var.resource_prefix}-gitlabrsssync-s3-policy-${var.environment}"
  description = "Policy to allow GitlabRSSSync to access S3 bucket for backups"
  policy      = data.aws_iam_policy_document.gitlabrsssync_s3.json
  tags        = local.tags
}

# Attach the policy to the role
resource "aws_iam_role_policy_attachment" "gitlabrsssync_s3" {
  role       = aws_iam_role.gitlabrsssync.name
  policy_arn = aws_iam_policy.gitlabrsssync_s3.arn
}

# If running in EC2, create an instance profile
resource "aws_iam_instance_profile" "gitlabrsssync" {
  name = "${var.resource_prefix}-gitlabrsssync-profile-${var.environment}"
  role = aws_iam_role.gitlabrsssync.name
  tags = local.tags
}

# Optional: If running in EKS with IRSA, create a service account
# resource "kubernetes_service_account" "gitlabrsssync" {
#   metadata {
#     name      = "gitlabrsssync"
#     namespace = var.kubernetes_namespace
#     annotations = {
#       "eks.amazonaws.com/role-arn" = aws_iam_role.gitlabrsssync.arn
#     }
#   }
# }
