output "vpc_id" {
  value = module.network.vpc_id
}

output "ecs_public_ip" {
  description = "Staging ECS EC2ホストのEIP（ALB配下。直接アクセスはできない）"
  value       = module.ecs_cluster.eip_public_ip
}

output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "frontend_url" {
  value = "https://${aws_route53_record.frontend.name}"
}

output "backend_url" {
  value = "https://${aws_route53_record.backend.name}"
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

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "ecr_push_commands" {
  description = "初回イメージ push の例（AWS CLI 要ログイン）"
  value       = <<-EOT
    aws ecr get-login-password --region ${var.region} | docker login --username AWS --password-stdin ${module.ecr.registry_id}.dkr.ecr.${var.region}.amazonaws.com
    docker tag soc-backend:local ${module.ecr.repository_urls["soc-backend"]}:${var.image_tag}
    docker push ${module.ecr.repository_urls["soc-backend"]}:${var.image_tag}
    docker tag soc-frontend:local ${module.ecr.repository_urls["soc-frontend"]}:${var.image_tag}
    docker push ${module.ecr.repository_urls["soc-frontend"]}:${var.image_tag}
    aws ecs update-service --cluster ${module.ecs_cluster.cluster_name} --service ${module.backend.service_name} --force-new-deployment --region ${var.region}
    aws ecs update-service --cluster ${module.ecs_cluster.cluster_name} --service ${module.frontend.service_name} --force-new-deployment --region ${var.region}
  EOT
}
