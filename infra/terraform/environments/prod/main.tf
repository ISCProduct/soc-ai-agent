terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

module "network" {
  source           = "../../modules/network"
  project_name     = var.project_name
  vpc_cidr         = var.vpc_cidr
  allowed_ssh_cidr = var.allowed_ssh_cidr
}

module "compute" {
  source            = "../../modules/compute"
  project_name      = var.project_name
  instance_type     = var.instance_type
  subnet_id         = module.network.public_subnet_id
  security_group_id = module.network.security_group_id
  key_name          = var.key_name
}

# Route 53 (既存のドメインを使用する場合)
data "aws_route53_zone" "selected" {
  name = var.domain_name
}

resource "aws_route53_record" "app" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = var.domain_name
  type    = "A"
  ttl     = "300"
  records = [module.compute.public_ip]
}

# API 用のサブドメイン
resource "aws_route53_record" "api" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = "api.${var.domain_name}"
  type    = "A"
  ttl     = "300"
  records = [module.compute.public_ip]
}

output "server_public_ip" {
  value = module.compute.public_ip
}
