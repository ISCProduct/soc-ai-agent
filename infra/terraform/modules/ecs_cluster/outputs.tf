output "cluster_id" {
  value = aws_ecs_cluster.this.id
}

output "cluster_name" {
  value = aws_ecs_cluster.this.name
}

output "capacity_provider_name" {
  value = aws_ecs_capacity_provider.this.name
}

output "eip_public_ip" {
  value = aws_eip.ecs.public_ip
}

output "eip_allocation_id" {
  value = aws_eip.ecs.id
}

output "asg_name" {
  value = aws_autoscaling_group.ecs.name
}
