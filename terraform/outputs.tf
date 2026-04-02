# Outputs are populated by individual resource files (ecr.tf, rds.tf, msk.tf, eks.tf).
# Run `terraform output` after apply to see all values.

output "ecr_repository_urls" {
  description = "ECR repository URLs per service"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}
