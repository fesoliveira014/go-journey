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
