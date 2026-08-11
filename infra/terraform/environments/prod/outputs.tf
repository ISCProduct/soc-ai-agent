output "vpc_id" {
  value = module.network.vpc_id
}

output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "frontend_url" {
  value = "https://${var.domain_name}"
}

output "backend_url" {
  value = "https://api.${var.domain_name}"
}

output "rds_endpoint" {
  value = module.rds.address
}

output "s3_bucket" {
  value = module.s3.bucket_id
}

output "db_secret_arn" {
  value = module.secrets.db_secret_arn
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.this.name
}

output "backend_service_name" {
  value = module.backend.service_name
}

output "frontend_service_name" {
  value = module.frontend.service_name
}
