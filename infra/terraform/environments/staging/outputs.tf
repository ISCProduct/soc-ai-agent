output "vpc_id" {
  value = module.network.vpc_id
}

output "asg_name" {
  description = "StagingアプリのAuto Scaling Group名（台数はEC2コンソール/CLIで確認）"
  value       = aws_autoscaling_group.app.name
}

output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "frontend_url" {
  value = var.enable_error_fallback ? "https://${aws_route53_record.frontend_primary[0].name}" : "https://${aws_route53_record.frontend[0].name}"
}

output "error_fallback_cloudfront_id" {
  value = var.enable_error_fallback ? module.error_fallback[0].cloudfront_distribution_id : null
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

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "ecr_push_commands" {
  description = "初回イメージ push の例（AWS CLI 要ログイン）。反映にはEC2再起動 or SSH上でdocker compose pull && up -d"
  value       = <<-EOT
    aws ecr get-login-password --region ${var.region} | docker login --username AWS --password-stdin ${module.ecr.registry_id}.dkr.ecr.${var.region}.amazonaws.com
    docker tag soc-backend:local ${module.ecr.repository_urls["soc-backend"]}:${var.image_tag}
    docker push ${module.ecr.repository_urls["soc-backend"]}:${var.image_tag}
    docker tag soc-frontend:local ${module.ecr.repository_urls["soc-frontend"]}:${var.image_tag}
    docker push ${module.ecr.repository_urls["soc-frontend"]}:${var.image_tag}
  EOT
}
