locals {
  tags = {
    Project = "soc-ai-agent"
    Env     = "production"
  }

  backend_secret_arns = compact(concat(
    [module.secrets.db_secret_arn],
    var.openai_secret_arn != "" ? [var.openai_secret_arn] : [],
    var.additional_secret_arns
  ))

  backend_secrets = concat(
    [
      {
        name      = "DB_HOST"
        valueFrom = "${module.secrets.db_secret_arn}:host::"
      },
      {
        name      = "DB_PORT"
        valueFrom = "${module.secrets.db_secret_arn}:port::"
      },
      {
        name      = "DB_NAME"
        valueFrom = "${module.secrets.db_secret_arn}:dbname::"
      },
      {
        name      = "DB_USER"
        valueFrom = "${module.secrets.db_secret_arn}:username::"
      },
      {
        name      = "DB_PASSWORD"
        valueFrom = "${module.secrets.db_secret_arn}:password::"
      }
    ],
    var.openai_secret_arn != "" ? [
      {
        name      = "OPENAI_API_KEY"
        valueFrom = var.openai_secret_arn
      }
    ] : []
  )
}

module "network" {
  source = "../../modules/network"

  project_name        = var.project_name
  vpc_cidr            = var.vpc_cidr
  azs                 = var.azs
  public_subnet_cidrs = var.public_subnet_cidrs
  allowed_http_cidrs  = var.allowed_http_cidrs
  enable_ssh          = var.enable_ssh
  allowed_ssh_cidrs   = var.allowed_ssh_cidrs
  tags                = local.tags
}

module "s3" {
  source = "../../modules/s3"

  project_name  = var.project_name
  force_destroy = false
  tags          = local.tags
}

module "rds" {
  source = "../../modules/rds"

  project_name            = var.project_name
  subnet_ids              = module.network.public_subnet_ids
  security_group_ids      = [module.network.rds_security_group_id]
  instance_class          = var.db_instance_class
  db_name                 = var.db_name
  deletion_protection     = var.rds_deletion_protection
  skip_final_snapshot     = var.rds_skip_final_snapshot
  backup_retention_period = var.rds_backup_retention_period
  tags                    = local.tags
}

module "secrets" {
  source = "../../modules/secrets"

  project_name = var.project_name
  db_host      = module.rds.address
  db_port      = module.rds.port
  db_name      = module.rds.db_name
  db_username  = module.rds.master_username
  db_password  = module.rds.master_password
  tags         = local.tags
}

module "ecs_cluster" {
  source = "../../modules/ecs_cluster"

  project_name      = var.project_name
  subnet_ids        = module.network.public_subnet_ids
  security_group_id = module.network.ecs_security_group_id
  instance_type     = var.instance_type
  desired_capacity  = var.ecs_desired_capacity
  min_size          = var.ecs_min_size
  max_size          = var.ecs_max_size
  tags              = local.tags
}

module "backend" {
  source = "../../modules/ecs_service"

  project_name           = var.project_name
  service_name           = "backend"
  cluster_id             = module.ecs_cluster.cluster_id
  capacity_provider_name = module.ecs_cluster.capacity_provider_name
  container_name         = "soc-backend"
  container_image        = var.backend_image
  container_port         = 8080
  host_port              = 8080
  cpu                    = 256
  memory                 = 512
  region                 = var.region
  s3_bucket_arn          = module.s3.bucket_arn
  secret_arns            = local.backend_secret_arns
  secrets                = local.backend_secrets
  environment = {
    APP_ENV       = "production"
    AWS_REGION    = var.region
    AWS_S3_BUCKET = module.s3.bucket_id
  }
  tags = local.tags
}

module "frontend" {
  source = "../../modules/ecs_service"

  project_name           = var.project_name
  service_name           = "frontend"
  cluster_id             = module.ecs_cluster.cluster_id
  capacity_provider_name = module.ecs_cluster.capacity_provider_name
  container_name         = "soc-frontend"
  container_image        = var.frontend_image
  container_port         = 3000
  host_port              = 3000
  cpu                    = 256
  memory                 = 512
  region                 = var.region
  environment = {
    APP_ENV                  = "production"
    NEXT_PUBLIC_API_BASE_URL = var.frontend_api_base_url != "" ? var.frontend_api_base_url : "http://api.${var.domain_name}:8080"
  }
  tags = local.tags
}

# --- ドメイン紐付け（既存 Route53 ホストゾーンを使用） ---
# ALB なし構成のため、独自ドメインでも :3000 / :8080 のポート付きアクセスになる。
# TLS/常時443化が必要になったら ALB + ACM の追加を検討する。
data "aws_route53_zone" "selected" {
  name = var.domain_name
}

resource "aws_route53_record" "frontend" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = var.domain_name
  type    = "A"
  ttl     = 300
  records = [module.ecs_cluster.eip_public_ip]
}

resource "aws_route53_record" "backend" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = "api.${var.domain_name}"
  type    = "A"
  ttl     = 300
  records = [module.ecs_cluster.eip_public_ip]
}
