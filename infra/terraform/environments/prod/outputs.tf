output "vpc_id" {
  value = module.network.vpc_id
}

output "ecs_public_ip" {
  description = "Production public IP (EIP). FE :3000 / BE :8080"
  value       = module.ecs_cluster.eip_public_ip
}

output "frontend_url" {
  value = "http://${var.domain_name}:3000"
}

output "backend_url" {
  value = "http://api.${var.domain_name}:8080"
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
  value = module.ecs_cluster.cluster_name
}

output "backend_service_name" {
  value = module.backend.service_name
}

output "frontend_service_name" {
  value = module.frontend.service_name
}
