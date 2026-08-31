locals {
  tags = {
    Project = "soc-ai-agent"
    Env     = "production"
  }

  frontend_domain = var.domain_name
  backend_domain  = "api.${var.domain_name}"

  backend_secret_arns = compact(concat(
    [module.secrets.db_secret_arn, aws_secretsmanager_secret.oauth.arn, aws_secretsmanager_secret.email.arn, aws_secretsmanager_secret.admin.arn, aws_secretsmanager_secret.openai.arn, aws_secretsmanager_secret.rag_internal.arn],
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
    ],
    [
      {
        name      = "RAG_INTERNAL_TOKEN"
        valueFrom = "${aws_secretsmanager_secret.rag_internal.arn}:rag_internal_token::"
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

  lifecycle {
    # openai_api_key/openai_secret_arnの両方が空のままapplyされると、OPENAI_API_KEYが
    # 空文字で本番backendが起動時にクラッシュする(過去に実際発生した障害)。
    # variable validationでのvar間参照はTerraform 1.9+が必要(このリポジトリの
    # required_version >= 1.5.0と非互換)なため、resourceのpreconditionで検証する。
    precondition {
      condition     = var.openai_api_key != "" || var.openai_secret_arn != ""
      error_message = "openai_api_key と openai_secret_arn のいずれかを設定してください(両方空だと本番backendが起動できません)。"
    }
  }
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

# rag-reviewはALBで外部公開せず、backend/chromaからのみ内部で到達できればよいため、
# VPC内限定のPrivate DNS Namespace(Cloud Map)でサービスディスカバリする。
resource "aws_service_discovery_private_dns_namespace" "internal" {
  name = "internal.${var.project_name}.local"
  vpc  = module.network.vpc_id
  tags = local.tags
}

resource "aws_service_discovery_service" "rag_review" {
  name = "rag-review"

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.internal.id
    dns_records {
      ttl  = 10
      type = "A"
    }
    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }

  tags = local.tags
}

resource "aws_service_discovery_service" "chroma" {
  name = "chroma"

  dns_config {
    namespace_id = aws_service_discovery_private_dns_namespace.internal.id
    dns_records {
      ttl  = 10
      type = "A"
    }
    routing_policy = "MULTIVALUE"
  }

  health_check_custom_config {
    failure_threshold = 1
  }

  tags = local.tags
}

# 最小権限の1ホップ許可チェーン: fargate(backend/frontend) -> rag-review -> chroma -> EFS。
# rag-review/chroma/EFSはALBで公開せず、それぞれ直前のホップからのみ到達可能にする。
resource "aws_security_group" "rag_review" {
  name_prefix = "${var.project_name}-rag-review-"
  description = "Managed by Terraform"
  vpc_id      = module.network.vpc_id

  ingress {
    from_port       = 9000
    to_port         = 9000
    protocol        = "tcp"
    security_groups = [module.network.fargate_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, {
    Name = "${var.project_name}-rag-review-sg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group" "chroma_fargate" {
  name_prefix = "${var.project_name}-chroma-"
  description = "Managed by Terraform"
  vpc_id      = module.network.vpc_id

  ingress {
    from_port       = 8000
    to_port         = 8000
    protocol        = "tcp"
    security_groups = [aws_security_group.rag_review.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, {
    Name = "${var.project_name}-chroma-sg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_security_group" "efs_chroma" {
  name_prefix = "${var.project_name}-efs-chroma-"
  description = "Managed by Terraform"
  vpc_id      = module.network.vpc_id

  ingress {
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.chroma_fargate.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, {
    Name = "${var.project_name}-efs-chroma-sg"
  })

  lifecycle {
    create_before_destroy = true
  }
}

# chroma(ベクトルDB)は独立したECS Fargateサービスとして動かす。Fargateタスクの
# ローカルストレージはタスク再起動で消えるため、EFSでエンベディングデータを永続化する。
resource "aws_efs_file_system" "chroma" {
  encrypted        = true
  throughput_mode  = "bursting"
  performance_mode = "generalPurpose"

  tags = merge(local.tags, {
    Name = "${var.project_name}-chroma"
  })
}

resource "aws_efs_mount_target" "chroma" {
  count = length(module.network.public_subnet_ids)

  file_system_id  = aws_efs_file_system.chroma.id
  subnet_id       = module.network.public_subnet_ids[count.index]
  security_groups = [aws_security_group.efs_chroma.id]
}

resource "aws_efs_access_point" "chroma" {
  file_system_id = aws_efs_file_system.chroma.id

  posix_user {
    uid = 0
    gid = 0
  }

  root_directory {
    path = "/chroma-data"
    creation_info {
      owner_uid   = 0
      owner_gid   = 0
      permissions = "0755"
    }
  }

  tags = merge(local.tags, {
    Name = "${var.project_name}-chroma"
  })
}

# rag-reviewサービス間の内部認証トークン(#1091台のRAGインフラ初回構築で導入)。
# Backend/RAGとも RAG_INTERNAL_TOKEN が一致しないとRAG側がリクエストを拒否する(fail-closed)。
resource "random_password" "rag_internal_token" {
  length  = 48
  special = false
}

resource "aws_secretsmanager_secret" "rag_internal" {
  name = "${var.project_name}/rag-internal"
  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "rag_internal" {
  secret_id = aws_secretsmanager_secret.rag_internal.id
  secret_string = jsonencode({
    rag_internal_token = random_password.rag_internal_token.result
  })
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
  # 本番反映の切り替え待機を短縮する(デプロイ頻度が高いため #運用実績)
  deregistration_delay = 10
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
    APP_ENV                     = "production"
    AWS_REGION                  = var.region
    AWS_S3_BUCKET               = module.s3.bucket_id
    EMAIL_PROVIDER              = "resend"
    EMAIL_FROM                  = "noreply@shukatsu-ai.jp"
    OPENAI_WEB_SEARCH_MODEL     = "gpt-4o-mini"
    OPENAI_COMPANY_SEARCH_MODEL = "gpt-4o-mini"
    OPENAI_HINTS_MODEL          = "gpt-4o-mini"
    # 未設定だとOAuthコールバックURLがlocalhost:8080にフォールバックし、
    # 本番でOAuthログインが機能しなくなる(実際に発生した障害)。
    BASE_URL = "https://${local.backend_domain}"
    APP_URL  = "https://${local.frontend_domain}"
    # 学園マルチテナントサブドメイン・admin.shukatsu-ai.jpからの直接アクセスを許可
    ALLOWED_ORIGINS = "https://${local.frontend_domain},https://*.${var.domain_name}"
    # 同一タスク内のredisサイドカーへlocalhost経由で接続(awsvpcモードはコンテナ間で
    # ネットワーク名前空間を共有するため)
    REDIS_URL = "redis://localhost:6379/0"
    # Cloud Map(Service Discovery)経由でrag-reviewタスクへ到達する
    RAG_REVIEW_URL = "http://rag-review.${aws_service_discovery_private_dns_namespace.internal.name}:9000"
  }
  extra_container_definitions = [
    {
      name      = "redis"
      image     = "redis:7-alpine"
      essential = false
      memory    = 64
      # コンテナのメモリハードリミット(64MiB)を超えるとOOM Killされるため、Redis自体の
      # maxmemoryはプロセスオーバーヘッド分の余裕を見て低めに設定する。noevictionにして、
      # 上限到達時はデータ(asynqのキュー・レートリミット用キー)を黙って捨てず書き込みエラーに
      # する(キュー済みジョブの消失より、書き込み失敗で気づける方を優先)。
      command      = ["redis-server", "--maxmemory", "48mb", "--maxmemory-policy", "noeviction"]
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

module "rag_review" {
  source = "../../modules/ecs_service_fargate"

  project_name                   = var.project_name
  service_name                   = "rag-review"
  cluster_id                     = aws_ecs_cluster.this.id
  subnet_ids                     = module.network.public_subnet_ids
  security_group_id              = aws_security_group.rag_review.id
  assign_public_ip               = true
  container_name                 = "rag-review"
  container_image                = var.rag_review_image
  container_port                 = 9000
  cpu                            = var.rag_review_cpu
  memory                         = var.rag_review_memory
  desired_count                  = var.rag_review_desired_count
  service_discovery_registry_arn = aws_service_discovery_service.rag_review.arn
  enable_execute_command         = true
  region                         = var.region
  s3_bucket_arn                  = module.s3.bucket_arn
  secret_arns                    = [aws_secretsmanager_secret.openai.arn, aws_secretsmanager_secret.rag_internal.arn]
  secrets = [
    {
      name      = "OPENAI_API_KEY"
      valueFrom = var.openai_api_key != "" ? "${aws_secretsmanager_secret.openai.arn}:openai_api_key::" : var.openai_secret_arn
    },
    {
      name      = "RAG_INTERNAL_TOKEN"
      valueFrom = "${aws_secretsmanager_secret.rag_internal.arn}:rag_internal_token::"
    }
  ]
  environment = {
    OPENAI_EMBEDDING_MODEL   = "text-embedding-3-small"
    OPENAI_HINTS_MODEL       = "gpt-4o-mini"
    OPENAI_HINTS_PARSE_MODEL = "gpt-4o"
    # chromaは独立サービス。Cloud Map経由で名前解決する
    CHROMA_HOST = "chroma.${aws_service_discovery_private_dns_namespace.internal.name}"
    CHROMA_PORT = "8000"
  }
  container_health_check = {
    command      = ["CMD-SHELL", "python3 -c \"import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:9000/healthz',timeout=3).status==200 else 1)\""]
    interval     = 30
    timeout      = 5
    retries      = 3
    start_period = 30
  }
  tags = local.tags
}

module "chroma" {
  source = "../../modules/ecs_service_fargate"

  project_name                   = var.project_name
  service_name                   = "chroma"
  cluster_id                     = aws_ecs_cluster.this.id
  subnet_ids                     = module.network.public_subnet_ids
  security_group_id              = aws_security_group.chroma_fargate.id
  assign_public_ip               = true
  container_name                 = "chroma"
  container_image                = "chromadb/chroma:0.6.3"
  container_port                 = 8000
  cpu                            = 512
  memory                         = 1024
  desired_count                  = var.rag_review_desired_count
  service_discovery_registry_arn = aws_service_discovery_service.chroma.arn
  enable_execute_command         = true
  region                         = var.region
  environment = {
    IS_PERSISTENT        = "TRUE"
    ANONYMIZED_TELEMETRY = "FALSE"
  }
  container_health_check = {
    command      = ["CMD-SHELL", "python3 -c \"import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8000/api/v1/heartbeat',timeout=3).status==200 else 1)\""]
    interval     = 30
    timeout      = 5
    retries      = 3
    start_period = 30
  }
  efs_volumes = [
    {
      name            = "chroma-data"
      file_system_id  = aws_efs_file_system.chroma.id
      file_system_arn = aws_efs_file_system.chroma.arn
      access_point_id = aws_efs_access_point.chroma.id
    }
  ]
  container_mount_points = [
    {
      sourceVolume  = "chroma-data"
      containerPath = "/chroma/chroma"
      readOnly      = false
    }
  ]
  tags = local.tags

  depends_on = [aws_efs_mount_target.chroma]
}

# backend/frontendのCPU使用率に応じたオートスケーリング(0〜2タスク)。
# 本番は既定停止(desired_count=0)方針のため、min_capacityも0にしておかないと
# オートスケーリングが1へ引き戻してしまう(terraform.tfvarsの運用コメント参照)。
resource "aws_appautoscaling_target" "backend" {
  max_capacity       = 2
  min_capacity       = var.backend_desired_count
  resource_id        = "service/${var.project_name}/backend"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"

  depends_on = [module.backend]
}

resource "aws_appautoscaling_policy" "backend_cpu" {
  name               = "${var.project_name}-backend-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.backend.resource_id
  scalable_dimension = aws_appautoscaling_target.backend.scalable_dimension
  service_namespace  = aws_appautoscaling_target.backend.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}

resource "aws_appautoscaling_target" "frontend" {
  max_capacity       = 2
  min_capacity       = var.frontend_desired_count
  resource_id        = "service/${var.project_name}/frontend"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"

  depends_on = [module.frontend]
}

resource "aws_appautoscaling_policy" "frontend_cpu" {
  name               = "${var.project_name}-frontend-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.frontend.resource_id
  scalable_dimension = aws_appautoscaling_target.frontend.scalable_dimension
  service_namespace  = aws_appautoscaling_target.frontend.service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
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
