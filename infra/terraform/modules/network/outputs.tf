output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "ecs_security_group_id" {
  value = aws_security_group.ecs.id
}

output "rds_security_group_id" {
  value = aws_security_group.rds.id
}

output "alb_security_group_id" {
  value = try(aws_security_group.alb[0].id, null)
}

output "fargate_security_group_id" {
  value = try(aws_security_group.fargate[0].id, null)
}
