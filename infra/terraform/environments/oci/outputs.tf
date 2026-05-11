output "server_public_ip" {
  description = "Compute Instance のパブリック IP アドレス"
  value       = module.compute.public_ip
}

output "db_endpoint" {
  description = "MySQL Database Service のエンドポイント"
  value       = module.database.db_endpoint
}

output "app_bucket_name" {
  description = "アプリ用 Object Storage バケット名"
  value       = module.storage.app_bucket_name
}

output "uploads_bucket_name" {
  description = "アップロード用 Object Storage バケット名"
  value       = module.storage.uploads_bucket_name
}
