output "repository_urls" {
  description = "name => repository URL (without tag)"
  value       = { for k, r in aws_ecr_repository.this : k => r.repository_url }
}

output "repository_arns" {
  value = { for k, r in aws_ecr_repository.this : k => r.arn }
}

output "registry_id" {
  value = try(values(aws_ecr_repository.this)[0].registry_id, "")
}
