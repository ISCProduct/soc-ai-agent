locals {
  tags = {
    Project = "soc-ai-agent"
    Env     = "production"
  }

  frontend_domain = var.domain_name
  backend_domain  = "api.${var.domain_name}"

  backend_secret_arns = compact(concat(
    [module.secrets.db_secret_arn, aws_secretsmanager_secret.oauth.arn, aws_secretsmanager_secret.email.arn, aws_secretsmanager_secret.admin.arn, aws_secretsmanager_secret.openai.arn],
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
    [
      {
        name      = "OPENAI_API_KEY"
        valueFrom = var.openai_api_key != "" ? "${aws_secretsmanager_secret.openai.arn}:openai_api_key::" : var.openai_secret_arn
      }
    ],
    [
      {
        name      = "GOOGLE_CLIENT_ID"
        valueFrom = "${aws_secretsmanager_secret.oauth.arn}:google_client_id::"
      },
      {
        name      = "GOOGLE_CLIENT_SECRET"
        valueFrom = "${aws_secretsmanager_secret.oauth.arn}:google_client_secret::"
      },
      {
        name      = "GITHUB_CLIENT_ID"
        valueFrom = "${aws_secretsmanager_secret.oauth.arn}:github_client_id::"
      },
      {
        name      = "GITHUB_CLIENT_SECRET"
        valueFrom = "${aws_secretsmanager_secret.oauth.arn}:github_client_secret::"
      }
    ],
    [
      {
        name      = "RESEND_API_KEY"
        valueFrom = "${aws_secretsmanager_secret.email.arn}:resend_api_key::"
      }
    ],
    [
      {
        name      = "ADMIN_SECRET"
        valueFrom = "${aws_secretsmanager_secret.admin.arn}:admin_secret::"
      },
      {
        name      = "USER_SECRET"
        valueFrom = "${aws_secretsmanager_secret.admin.arn}:user_secret::"
      },
      {
        name      = "OAUTH_STATE_SECRET"
        valueFrom = "${aws_secretsmanager_secret.admin.arn}:oauth_state_secret::"
      },
      {
        name      = "TOKEN_ENCRYPTION_KEY"
        valueFrom = "${aws_secretsmanager_secret.admin.arn}:token_encryption_key::"
      }
    ]
  )
}

# 本番未起動状態の初回構築時に見落とされていた認証系シークレット。
# staging(app_user_data.sh.tftpl)と同じくrandom_passwordで自動生成する。
# ADMIN_SECRETのみCI(sync-whats-newジョブ)から既知の値で呼べる必要があるため
# var.admin_secret(固定値、stagingと同じ値を設定)を使う。
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

# Google/GitHub OAuthクライアント認証情報(DB同様、Secrets Managerで管理しECSタスク実行ロール経由で注入)
resource "aws_secretsmanager_secret" "oauth" {
  name = "${var.project_name}/oauth"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "oauth" {
  secret_id = aws_secretsmanager_secret.oauth.id
  secret_string = jsonencode({
    google_client_id     = var.google_client_id
    google_client_secret = var.google_client_secret
    github_client_id     = var.github_client_id
    github_client_secret = var.github_client_secret
  })
}

# Resend(メール送信)APIキー(#756: EMAIL_PROVIDER未設定でもRESEND_API_KEYがあれば自動選択される)
resource "aws_secretsmanager_secret" "email" {
  name = "${var.project_name}/email"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "email" {
  secret_id = aws_secretsmanager_secret.email.id
  secret_string = jsonencode({
    resend_api_key = var.resend_api_key
  })
}

# 管理者認証シークレット(sync-whats-newジョブ等、CIからのサービス間呼び出しに使用)
resource "aws_secretsmanager_secret" "admin" {
  name = "${var.project_name}/admin"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "admin" {
  secret_id = aws_secretsmanager_secret.admin.id
  secret_string = jsonencode({
    admin_secret         = var.admin_secret
    user_secret          = random_password.user_secret.result
    oauth_state_secret   = random_password.oauth_state_secret.result
    token_encryption_key = random_id.token_encryption_key.hex
  })
}

# OpenAI APIキー(DB/OAuth同様、Secrets Managerで管理しECSタスク実行ロール経由で注入)
resource "aws_secretsmanager_secret" "openai" {
  name = "${var.project_name}/openai"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "openai" {
  secret_id = aws_secretsmanager_secret.openai.id
  secret_string = jsonencode({
    openai_api_key = var.openai_api_key
  })
}

module "network" {
  source = "../../modules/network"

  project_name        = var.project_name
  vpc_cidr            = var.vpc_cidr
  azs                 = var.azs
  public_subnet_cidrs = var.public_subnet_cidrs
  allowed_http_cidrs  = var.allowed_http_cidrs
  enable_alb          = true
  alb_ingress_cidrs   = var.allowed_http_cidrs
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

# Fargateはインスタンス/ASGを持たないため、クラスタはこの1リソースのみ
resource "aws_ecs_cluster" "this" {
  name = var.project_name

  setting {
    name  = "containerInsights"
    value = "disabled"
  }

  tags = merge(local.tags, {
    Name = var.project_name
  })
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
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
  target_type          = "ip"
  # 学園マルチテナント(<学園slug>.shukatsu-ai.jp)とadmin.shukatsu-ai.jp用のワイルドカードSAN
  additional_san_domains = ["*.${var.domain_name}"]
  tags                   = local.tags
}

module "backend" {
  source = "../../modules/ecs_service_fargate"

  project_name      = var.project_name
  service_name      = "backend"
  cluster_id        = aws_ecs_cluster.this.id
  subnet_ids        = module.network.public_subnet_ids
  security_group_id = module.network.fargate_security_group_id
  assign_public_ip  = true
  container_name    = "soc-backend"
  container_image   = var.backend_image
  container_port    = 8080
  cpu               = var.backend_cpu
  memory            = var.backend_memory
  desired_count     = var.backend_desired_count
  target_group_arn  = module.alb.backend_target_group_arn
  region            = var.region
  s3_bucket_arn     = module.s3.bucket_arn
  secret_arns       = local.backend_secret_arns
  secrets           = local.backend_secrets
  environment = {
    APP_ENV        = "production"
    AWS_REGION     = var.region
    AWS_S3_BUCKET  = module.s3.bucket_id
    EMAIL_PROVIDER = "resend"
    EMAIL_FROM     = "noreply@shukatsu-ai.jp"
    # 未設定だとOAuthコールバックURLがlocalhost:8080にフォールバックし、
    # 本番でOAuthログインが機能しなくなる(実際に発生した障害)。
    BASE_URL = "https://${local.backend_domain}"
    APP_URL  = "https://${local.frontend_domain}"
    # 学園マルチテナントサブドメイン・admin.shukatsu-ai.jpからの直接アクセスを許可
    ALLOWED_ORIGINS = "https://${local.frontend_domain},https://*.${var.domain_name}"
    # 同一タスク内のredisサイドカーへlocalhost経由で接続(awsvpcモードはコンテナ間で
    # ネットワーク名前空間を共有するため)
    REDIS_URL = "redis://localhost:6379/0"
  }
  extra_container_definitions = [
    {
      name         = "redis"
      image        = "redis:7-alpine"
      essential    = false
      memory       = 64
      portMappings = []
    }
  ]
  tags = local.tags
}

module "frontend" {
  source = "../../modules/ecs_service_fargate"

  project_name      = var.project_name
  service_name      = "frontend"
  cluster_id        = aws_ecs_cluster.this.id
  subnet_ids        = module.network.public_subnet_ids
  security_group_id = module.network.fargate_security_group_id
  assign_public_ip  = true
  container_name    = "soc-frontend"
  container_image   = var.frontend_image
  container_port    = 3000
  cpu               = var.frontend_cpu
  memory            = var.frontend_memory
  desired_count     = var.frontend_desired_count
  target_group_arn  = module.alb.frontend_target_group_arn
  region            = var.region
  environment = {
    APP_ENV = "production"
    # NEXT_PUBLIC_*はクライアントバンドルにビルド時埋め込みされるため実行時のこの値では
    # login-page.tsx等のクライアントコードには効かない(GitHub Actions側のdocker build
    # --build-argで焼き込む必要がある)。ここではサーバー側コード(middleware.tsの
    # セッションリフレッシュ等)がprocess.envを実行時に読む経路のために設定する。
    BACKEND_URL             = var.frontend_api_base_url != "" ? var.frontend_api_base_url : "https://${local.backend_domain}"
    NEXT_PUBLIC_BACKEND_URL = var.frontend_api_base_url != "" ? var.frontend_api_base_url : "https://${local.backend_domain}"
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

# 学園マルチテナント(<学園slug>.shukatsu-ai.jp)とadmin.shukatsu-ai.jp用のワイルドカードDNS。
# デフォルトアクション(frontendへforward)がそのまま使われるため、ALB側のルーティング追加は不要。
resource "aws_route53_record" "wildcard" {
  zone_id = data.aws_route53_zone.selected.zone_id
  name    = "*.${var.domain_name}"
  type    = "A"

  alias {
    name                   = module.alb.alb_dns_name
    zone_id                = module.alb.alb_zone_id
    evaluate_target_health = true
  }
}
