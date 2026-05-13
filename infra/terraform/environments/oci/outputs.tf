output "server_public_ip" {
  description = "Compute Instance のパブリック IP アドレス"
  value       = module.compute.public_ip
}
