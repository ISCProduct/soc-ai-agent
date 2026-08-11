locals {
  tags = {
    Project = "soc-ai-agent"
    Env     = "staging"
  }

  frontend_domain = "${var.staging_subdomain}.${var.domain_name}"
  backend_domain  = "${var.staging_api_subdomain}.${var.domain_name}"

  # 空なら Terraform 作成の ECR + image_tag を使う
  backend_image  = var.backend_image != "" ? var.backend_image : "${module.ecr.repository_urls["soc-backend"]}:${var.image_tag}"
  frontend_image = var.frontend_image != "" ? var.frontend_image : "${module.ecr.repository_urls["soc-frontend"]}:${var.image_tag}"

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
  enable_alb          = true
  alb_ingress_cidrs   = var.allowed_http_cidrs
  tags                = local.tags
}

module "ecr" {
  source = "../../modules/ecr"

  repository_names     = var.ecr_repository_names
  force_delete         = var.ecr_force_delete
  lifecycle_keep_count = var.ecr_lifecycle_keep_count
  tags                 = local.tags
}

module "s3" {
  source = "../../modules/s3"

  project_name  = var.project_name
  force_destroy = true
  tags          = local.tags
}

module "rds" {
  source = "../../modules/rds"

  project_name       = var.project_name
  subnet_ids         = module.network.public_subnet_ids
  security_group_ids = [module.network.rds_security_group_id]
  instance_class     = var.db_instance_class
  db_name            = var.db_name
  tags               = local.tags
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
  desired_capacity  = 1
  min_size          = 1
  max_size          = 2
  tags              = local.tags
}

# --- ドメイン紐付け（既存 Route53 ホストゾーンを使用） ---
data "aws_route53_zone" "selected" {
  name = var.domain_name
}

module "alb" {
  source = "../../modules/alb"

  project_name         = var.project_name
  vpc_id               = module.network.vpc_id
  subnet_ids           = module.network.public_subnet_ids
  security_group_id    = module.network.alb_security_group_id
  route53_zone_id      = data.aws_route53_zone.selected.zone_id
  frontend_domain_name = local.frontend_domain
  backend_domain_name  = local.backend_domain
  frontend_target_port = 3000
  backend_target_port  = 8080
  target_type          = "instance"
  tags                 = local.tags
}

module "backend" {
  source = "../../modules/ecs_service"

  project_name           = var.project_name
  service_name           = "backend"
  cluster_id             = module.ecs_cluster.cluster_id
  capacity_provider_name = module.ecs_cluster.capacity_provider_name
  container_name         = "soc-backend"
  container_image        = local.backend_image
  container_port         = 8080
  target_group_arn       = module.alb.backend_target_group_arn
  cpu                    = 256
  memory                 = 512
  region                 = var.region
  s3_bucket_arn          = module.s3.bucket_arn
  secret_arns            = local.backend_secret_arns
  secrets                = local.backend_secrets
  environment = {
    APP_ENV       = "staging"
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
  container_image        = local.frontend_image
  container_port         = 3000
  target_group_arn       = module.alb.frontend_target_group_arn
  cpu                    = 256
  memory                 = 512
  region                 = var.region
  environment = {
    APP_ENV                  = "staging"
    NEXT_PUBLIC_API_BASE_URL = var.frontend_api_base_url != "" ? var.frontend_api_base_url : "https://${local.backend_domain}"
  }
  tags = local.tags
}

resource "aws_route53_record" "frontend" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = local.frontend_domain
  type    = "A"

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "backend" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = local.backend_domain
  type    = "A"

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}
