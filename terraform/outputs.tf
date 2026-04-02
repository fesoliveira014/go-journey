# Outputs are populated by individual resource files (ecr.tf, rds.tf, msk.tf, eks.tf).
# Run `terraform output` after apply to see all values.

output "ecr_repository_urls" {
  description = "ECR repository URLs per service"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}

output "rds_endpoints" {
  description = "RDS instance endpoints per service"
  value       = { for k, v in aws_db_instance.databases : k => v.endpoint }
}

output "rds_master_password_secret_arns" {
  description = "Secrets Manager ARNs for RDS master passwords"
  value       = { for k, v in aws_db_instance.databases : k => v.master_user_secret[0].secret_arn }
}

output "msk_bootstrap_brokers_tls" {
  description = "MSK bootstrap broker string (TLS)"
  value       = aws_msk_cluster.main.bootstrap_brokers_tls
}

output "cluster_name" {
  description = "EKS cluster name"
  value       = module.eks.cluster_name
}

output "cluster_endpoint" {
  description = "EKS cluster API endpoint"
  value       = module.eks.cluster_endpoint
}

output "cluster_certificate_authority" {
  description = "EKS cluster CA certificate (base64)"
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "oidc_provider_arn" {
  description = "EKS OIDC provider ARN (for IRSA)"
  value       = module.eks.oidc_provider_arn
}

output "github_actions_role_arn" {
  description = "IAM role ARN for GitHub Actions OIDC federation"
  value       = aws_iam_role.github_actions.arn
}

output "app_domain" {
  description = "Application domain name"
  value       = var.domain_name
}

output "certificate_arn" {
  description = "ACM certificate ARN"
  value       = var.domain_name != "" ? aws_acm_certificate.app[0].arn : ""
}
