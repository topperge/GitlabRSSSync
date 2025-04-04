variable "aws_region" {
  description = "AWS region where resources will be created"
  type        = string
  default     = "us-east-1"
}

variable "bucket_prefix" {
  description = "Prefix for S3 bucket name"
  type        = string
  default     = "gitlabrsssync-backup"
}

variable "resource_prefix" {
  description = "Prefix for IAM and other resources"
  type        = string
  default     = "gitlabrsssync"
}

variable "environment" {
  description = "Environment name (e.g., dev, staging, prod)"
  type        = string
  default     = "dev"
  validation {
    condition     = contains(["dev", "test", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, test, staging, prod"
  }
}

variable "tags" {
  description = "Default tags to apply to all resources"
  type        = map(string)
  default = {
    ManagedBy = "terraform"
    Project   = "GitlabRSSSync"
  }
}

variable "kubernetes_namespace" {
  description = "Kubernetes namespace for GitlabRSSSync deployment"
  type        = string
  default     = "gitlabrsssync"
}

variable "enable_versioning" {
  description = "Enable versioning for S3 bucket"
  type        = bool
  default     = true
}

variable "lifecycle_rules_enabled" {
  description = "Enable lifecycle rules for S3 bucket"
  type        = bool
  default     = true
}

variable "noncurrent_version_expiration_days" {
  description = "Number of days to keep noncurrent versions before expiration"
  type        = number
  default     = 30
}

variable "standard_ia_transition_days" {
  description = "Number of days before transitioning objects to STANDARD_IA storage class"
  type        = number
  default     = 30
}

variable "glacier_transition_days" {
  description = "Number of days before transitioning objects to GLACIER storage class"
  type        = number
  default     = 90
}

variable "object_expiration_days" {
  description = "Number of days before objects expire"
  type        = number
  default     = 365
}
