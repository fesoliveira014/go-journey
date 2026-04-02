variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name used as prefix for all resources"
  type        = string
  default     = "library-system"
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "library-system"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "github_repo" {
  description = "GitHub repository in owner/repo format for OIDC federation"
  type        = string
}
