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
  rag_image      = var.rag_image != "" ? var.rag_image : "${module.ecr.repository_urls["soc-rag-review"]}:${var.image_tag}"

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

# --- ドメイン紐付け（既存 Route53 ホストゾーンを使用） ---
data "aws_route53_zone" "selected" {
  name = var.domain_name
}

module "alb" {
  source = "../../modules/alb"

  project_name               = var.project_name
  vpc_id                     = module.network.vpc_id
  subnet_ids                 = module.network.public_subnet_ids
  security_group_id          = module.network.alb_security_group_id
  route53_zone_id            = data.aws_route53_zone.selected.zone_id
  frontend_domain_name       = local.frontend_domain
  backend_domain_name        = local.backend_domain
  frontend_target_port       = 3000
  backend_target_port        = 8080
  frontend_health_check_path = "/edge-healthz"
  target_type                = "instance"
  tags                       = local.tags
}

# ALB ターゲット全滅時（EC2停止等）に Route53 がフェイルオーバーする OGP 付き静的エラーページ
# ponytail: cloudfront:Create* 権限が無い環境では enable_error_fallback=false のまま
module "error_fallback" {
  count  = var.enable_error_fallback ? 1 : 0
  source = "../../modules/error_fallback"

  providers = {
    aws.us_east_1 = aws.us_east_1
  }

  project_name             = var.project_name
  env                      = "staging"
  domain_name              = local.frontend_domain
  aliases                  = [local.frontend_domain]
  route53_zone_id          = data.aws_route53_zone.selected.zone_id
  service_unavailable_html = file("${path.module}/../../../static/service-unavailable.html")
  tags                     = local.tags
}

# --- アプリ実行用EC2（プレーンEC2 + Docker Compose） ---
# ECS on EC2(IAMロール必須)への移行は別途検討として、ECR pull・S3アクセスは
# インスタンスプロファイル経由のIAMロールで行う（#829: 旧・長期アクセスキー埋め込みから移行）。
resource "aws_iam_role" "app" {
  name = "${var.project_name}-app-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

data "aws_caller_identity" "current" {}

resource "aws_iam_role_policy" "app" {
  name = "${var.project_name}-app-policy"
  role = aws_iam_role.app.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "EcrAuth"
        Effect   = "Allow"
        Action   = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Sid    = "EcrPull"
        Effect = "Allow"
        Action = [
          "ecr:BatchGetImage",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchCheckLayerAvailability",
        ]
        Resource = values(module.ecr.repository_arns)
      },
      {
        Sid    = "AppS3Access"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket",
        ]
        Resource = [module.s3.bucket_arn, "${module.s3.bucket_arn}/*"]
      },
      {
        # Discord連携(/api/discord/interactions)から本番の「指定日終日起動」日付リストを
        # 読み書きするための権限。対象パラメータは本番プロジェクト名(soc-app)固定。
        Sid    = "ProdUptimeSsmAccess"
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:PutParameter",
        ]
        Resource = "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/prod-uptime-dates"
      },
    ]
  })
}

resource "aws_iam_instance_profile" "app" {
  name = "${var.project_name}-app-profile"
  role = aws_iam_role.app.name
}

resource "random_password" "user_secret" {
  length  = 48
  special = false
}

resource "random_password" "oauth_state_secret" {
  length  = 32
  special = false
}

resource "random_id" "token_encryption_key" {
  byte_length = 32
}

data "aws_ami" "app" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
}

resource "aws_key_pair" "app" {
  key_name   = "${var.project_name}-app"
  public_key = var.ssh_public_key
}

resource "aws_launch_template" "app" {
  name_prefix            = "${var.project_name}-app-"
  image_id               = data.aws_ami.app.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.app.key_name
  vpc_security_group_ids = [module.network.ecs_security_group_id]

  iam_instance_profile {
    name = aws_iam_instance_profile.app.name
  }

  block_device_mappings {
    device_name = "/dev/sda1"
    ebs {
      # backend/frontend/rag-review の同時 pull で 20GB は不足。
      # 30GB でも旧イメージ稼働中の pull で逼迫するため 40GB。
      volume_size = 40
      volume_type = "gp3"
    }
  }

  user_data = base64encode(templatefile("${path.module}/app_user_data.sh.tftpl", {
    aws_region               = var.region
    backend_image            = local.backend_image
    frontend_image           = local.frontend_image
    rag_image                = local.rag_image
    db_host                  = module.rds.address
    db_port                  = module.rds.port
    db_name                  = module.rds.db_name
    db_user                  = module.rds.master_username
    db_password              = module.rds.master_password
    s3_bucket                = module.s3.bucket_id
    openai_api_key           = var.openai_api_key_plain
    openai_model             = var.openai_model
    resend_api_key           = var.resend_api_key_plain
    google_client_id         = var.google_client_id
    google_client_secret     = var.google_client_secret
    github_client_id         = var.github_client_id
    github_client_secret     = var.github_client_secret
    backend_domain           = local.backend_domain
    frontend_domain          = local.frontend_domain
    user_secret              = random_password.user_secret.result
    admin_secret             = var.admin_secret_plain
    discord_public_key       = var.discord_public_key
    discord_allowed_role_id  = var.discord_allowed_role_id
    oauth_state_secret       = random_password.oauth_state_secret.result
    token_encryption_key     = random_id.token_encryption_key.hex
    edge_nginx_conf          = file("${path.module}/../../../nginx/staging-edge.conf")
    service_unavailable_html = file("${path.module}/../../../static/service-unavailable.html")
    service_starting_html    = file("${path.module}/../../../static/service-starting.html")
  }))

  tag_specifications {
    resource_type = "instance"
    tags          = merge(local.tags, { Name = "${var.project_name}-app" })
  }

  tags = local.tags
}

# 平常時は最小構成(desired=min=1)、負荷試験時にCPU使用率でオートスケールする。
# ponytail: health_check_type=EC2。アプリのヘルスチェック応答が安定してから
#           ELBに切り替える(不安定なままだと正常インスタンスまで入れ替わり続ける)。
resource "aws_autoscaling_group" "app" {
  name                = "${var.project_name}-app"
  vpc_zone_identifier = module.network.public_subnet_ids
  min_size            = var.asg_min_size
  max_size            = var.asg_max_size
  desired_capacity    = var.asg_desired_capacity
  health_check_type   = "EC2"

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }

  target_group_arns = [
    module.alb.backend_target_group_arn,
    module.alb.frontend_target_group_arn,
  ]

  instance_refresh {
    strategy = "Rolling"
  }

  # デプロイ後1時間で自動停止/次回デプロイ時に自動起動する運用のため、
  # CIが変更するdesired_capacityをterraform applyで巻き戻さない
  lifecycle {
    ignore_changes = [desired_capacity]
  }

  tag {
    key                 = "Name"
    value               = "${var.project_name}-app"
    propagate_at_launch = true
  }
  tag {
    key                 = "Project"
    value               = local.tags.Project
    propagate_at_launch = true
  }
  tag {
    key                 = "Env"
    value               = local.tags.Env
    propagate_at_launch = true
  }
}

resource "aws_autoscaling_policy" "cpu_target_tracking" {
  name                   = "${var.project_name}-cpu-target-tracking"
  autoscaling_group_name = aws_autoscaling_group.app.name
  policy_type            = "TargetTrackingScaling"

  target_tracking_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ASGAverageCPUUtilization"
    }
    target_value = var.asg_target_cpu_percent
  }
}

resource "aws_security_group_rule" "app_backend_from_alb" {
  type                     = "ingress"
  description              = "ALB to backend (plain EC2, no ECS)"
  from_port                = 8080
  to_port                  = 8080
  protocol                 = "tcp"
  security_group_id        = module.network.ecs_security_group_id
  source_security_group_id = module.network.alb_security_group_id
}

resource "aws_security_group_rule" "app_frontend_from_alb" {
  type                     = "ingress"
  description              = "ALB to frontend (plain EC2, no ECS)"
  from_port                = 3000
  to_port                  = 3000
  protocol                 = "tcp"
  security_group_id        = module.network.ecs_security_group_id
  source_security_group_id = module.network.alb_security_group_id
}

resource "aws_route53_record" "frontend" {
  count   = var.enable_error_fallback ? 0 : 1
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = local.frontend_domain
  type    = "A"

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}

# enable_error_fallback=true: ALB → CloudFront(S3) フェイルオーバー
resource "aws_route53_record" "frontend_primary" {
  count   = var.enable_error_fallback ? 1 : 0
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = local.frontend_domain
  type    = "A"

  set_identifier = "primary"

  failover_routing_policy {
    type = "PRIMARY"
  }

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "frontend_secondary" {
  count   = var.enable_error_fallback ? 1 : 0
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = local.frontend_domain
  type    = "A"

  set_identifier = "secondary"

  failover_routing_policy {
    type = "SECONDARY"
  }

  alias {
    name                   = module.error_fallback[0].cloudfront_domain_name
    zone_id                = module.error_fallback[0].cloudfront_hosted_zone_id
    evaluate_target_health = false
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
