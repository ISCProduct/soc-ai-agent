output "server_public_ip" {
  description = "Compute Instance のパブリック IP アドレス"
  value       = module.compute.public_ip
}

output "db_endpoint" {
  description = "MySQL Database Service のエンドポイント"
  value       = module.database.db_endpoint
}

