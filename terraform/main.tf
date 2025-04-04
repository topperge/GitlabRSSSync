terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  required_version = ">= 1.2.0"
}

provider "aws" {
  region = var.aws_region

  # Add additional provider configuration if needed
  # For example, assume_role for cross-account access, etc.
}

# Create random suffix for globally unique S3 bucket name
resource "random_string" "bucket_suffix" {
  length  = 8
  special = false
  upper   = false
}

# Local variables
locals {
  bucket_name = "${var.bucket_prefix}-${random_string.bucket_suffix.result}"
  environment = var.environment
  tags = merge(
    var.tags,
    {
      Name        = local.bucket_name
      Environment = local.environment
      Terraform   = "true"
      Service     = "GitlabRSSSync"
    }
  )
}
