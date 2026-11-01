variable "region" {
  type    = string
  default = "ap-northeast-1"
}

variable "project_name" {
  type    = string
  default = "soc-stg"
}

variable "vpc_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

variable "azs" {
  type    = list(string)
  default = ["ap-northeast-1a", "ap-northeast-1c"]
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.10.1.0/24", "10.10.2.0/24"]
}

variable "allowed_http_cidrs" {
  type        = list(string)
  description = "Restrict to office/VPN CIDRs in real use when possible"
  default     = ["0.0.0.0/0"]
}

variable "enable_ssh" {
  type    = bool
  default = false
}

variable "allowed_ssh_cidrs" {
  type    = list(string)
  default = []
}

variable "ssh_public_key" {
  type        = string
  description = "アプリEC2にSSHで入って.env更新/再起動するための公開鍵(インスタンス再作成を避けるため)"
  default     = ""
}

variable "instance_type" {
  type = string
  # amd64（CIビルドと同一アーキテクチャに揃え、QEMUクロスビルドを避ける）
  default = "t3.small"
}

# --- 平常時は最小構成、負荷試験時にオートスケールするための設定 ---
variable "asg_min_size" {
  type        = number
  description = "平常時の最小台数"
  default     = 1
}

variable "asg_max_size" {
  type        = number
  description = "負荷試験時に許容する最大台数"
  default     = 3
}

variable "asg_desired_capacity" {
  type        = number
  description = "平常時の希望台数(最小構成)"
  default     = 1
}

variable "asg_target_cpu_percent" {
  type        = number
  description = "ターゲット追跡スケーリングのCPU使用率目標値"
  default     = 60
}

variable "enable_error_fallback" {
  type        = bool
  description = "CloudFront+S3の503フェイルオーバー（cloudfront:* IAM権限が必要）"
  default     = false
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "db_name" {
  type    = string
  default = "app_db"
}

variable "backend_image" {
  type        = string
  description = "上書き用。空なら module.ecr の soc-backend + image_tag"
  default     = ""
}

variable "frontend_image" {
  type        = string
  description = "上書き用。空なら module.ecr の soc-frontend + image_tag"
  default     = ""
}

variable "rag_image" {
  type        = string
  description = "上書き用。空なら module.ecr の soc-rag-review + image_tag"
  default     = ""
}

variable "image_tag" {
  type        = string
  description = "ECR に push するタグ（backend_image/frontend_image 未指定時）"
  default     = "staging"
}

variable "ecr_repository_names" {
  type    = list(string)
  default = ["soc-backend", "soc-frontend", "soc-rag-review"]
}

variable "ecr_force_delete" {
  type        = bool
  description = "staging では destroy しやすく true 推奨"
  default     = true
}

variable "ecr_lifecycle_keep_count" {
  type    = number
  default = 20
}

variable "openai_secret_arn" {
  type        = string
  description = "Secrets Manager ARN for OPENAI_API_KEY (staging recommended)"
  default     = ""
}

variable "frontend_api_base_url" {
  type        = string
  description = "NEXT_PUBLIC_API_BASE_URL. Leave empty to use http://<eip>:8080"
  default     = ""
}

variable "additional_secret_arns" {
  type        = list(string)
  description = "Extra secret ARNs the backend execution role may read"
  default     = []
}

variable "openai_api_key_plain" {
  type        = string
  description = "OpenAI APIキー(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定でも apply 可"
  sensitive   = true
  default     = ""
}

variable "gbizinfo_api_key" {
  type        = string
  description = "gBizINFO Web-APIのアクセストークン(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定時はgBizINFO連携が無効になり、企業情報の取得がweb_searchに寄る"
  sensitive   = true
  default     = ""
}

variable "gbizinfo_base_url" {
  type        = string
  description = "gBizINFOのベースURL。提供されているのはv1で、コード側が /v1/hojin/... を付ける"
  default     = "https://info.gbiz.go.jp/hojin"
}

variable "houjin_bangou_app_id" {
  type        = string
  description = "国税庁 法人番号システムWeb-APIのアプリケーションID(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定時は法人番号の自動特定だけが無効になる"
  sensitive   = true
  default     = ""
}

variable "openai_model" {
  type        = string
  description = "チャット採点・要約・面接進行など汎用タスクのデフォルトモデル。未設定時はコード側でgpt-4o-miniにフォールバックするが、環境変数の設定漏れ・実験用設定の残留による意図しない高コストモデル化を防ぐため明示的に固定する"
  default     = "gpt-4o-mini"
}

variable "resend_api_key_plain" {
  type        = string
  description = "Resend APIキー(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定時はEMAIL_PROVIDERがlogにフォールバックする"
  sensitive   = true
  default     = ""
}

variable "admin_secret_plain" {
  type        = string
  description = "管理者トークン(X-Admin-Token)のHMAC鍵(平文)。CIには渡さない(#1158: CIが叩く共有シークレットは service_secret_plain)"
  sensitive   = true
  default     = ""
}

variable "service_secret_plain" {
  type        = string
  description = "CIからのサービス間呼び出し(X-Admin-Secret)用の共有シークレット(平文)。GitHubのリポジトリシークレット SERVICE_SECRET と同じ値にする。admin_secret_plain とは必ず別値にする(#1158)"
  sensitive   = true

  # 既定値を持たせない。空のまま apply すると SERVICE_SECRET も aws_ssm_parameter も空になり、
  # CI の取り込みが 503 で拒否される（CI 側は警告で流すので気づきにくい）。
  # plan の時点で止めて、tfvars への設定漏れをその場で分かるようにする。
  #
  # 空文字だけでは足りない。terraform.tfvars.example をそのままコピーして apply すると
  # 例示値が本物の SERVICE_SECRET として .env と SSM へ入り、リポジトリを読める第三者が
  # /api/admin/whats-new/ingest と /api/admin/training/stats を叩けてしまう。
  # 例示値は Backend/.env.example と同じ change-me-service-secret に揃えてあり
  # (Backend/internal/config/config.go の serviceSecretPlaceholder と同値)、ここで弾く。
  # 山括弧を含む値も `<...>` 形のプレースホルダーなので弾く。
  # 対応関係は automation/test/service-secret-placeholder-test.sh が固定している。
  validation {
    condition     = length(var.service_secret_plain) > 0
    error_message = "service_secret_plain は必須です。GitHub のリポジトリシークレット SERVICE_SECRET と同じ値を tfvars に設定してください(#1158)。"
  }

  validation {
    condition     = var.service_secret_plain != "change-me-service-secret" && !can(regex("[<>]", var.service_secret_plain))
    error_message = "service_secret_plain がプレースホルダーのままです。terraform.tfvars.example の値をコピーしただけだと、リポジトリを読める第三者がサービス間エンドポイントを叩けます(#1158)。"
  }

  # 改行は引用では防げない。この値は .env の1行として書かれ、デプロイ時の継ぎ足しも
  # `echo "SERVICE_SECRET=$..." >> .env` の1行なので、改行が入ると2行目以降が
  # 別の環境変数として読まれる（CI と値が一致せず、取り込みが静かに 403 になる）。
  # SSM へ載る前に plan で止める。
  validation {
    condition     = !can(regex("[\r\n]", var.service_secret_plain))
    error_message = "service_secret_plain に改行を含めないでください。この値は .env の1行として書き込まれます(#1158)。"
  }
}

# AI面接の音声認識モデル。既定はコード側と同じ gpt-transcribe。
# 実音声の誤り率は mini 23% / gpt-transcribe 18%。費用を優先するなら
# gpt-4o-mini-transcribe に戻せる（問題発話だけ gpt-transcribe へ再送される）。
variable "openai_whisper_model" {
  description = "AI面接のSTTモデル。費用を下げるなら gpt-4o-mini-transcribe に変更する"
  type        = string
  default     = "gpt-transcribe"
}

variable "discord_public_key" {
  type        = string
  description = "Discord Application の Public Key。Interactions Endpoint署名検証に使用。未設定時はDiscord連携機能を無効化"
  default     = ""
}

variable "discord_allowed_role_id" {
  type        = string
  description = "本番の起動/停止コマンド(/prod, /prod-uptime)を実行できるDiscordロールID"
  default     = ""
}

variable "github_dispatch_token" {
  type        = string
  sensitive   = true
  description = "Discordの /prod から prod-uptime-scheduler.yml を即時起動するためのGitHubトークン(actions:write)。未設定でも機能は動くが、反映は次の毎時実行まで待つことになる"
  default     = ""
}

variable "github_dispatch_repo" {
  type        = string
  description = "workflow_dispatch の対象リポジトリ(owner/repo)"
  default     = "ISCProduct/soc-ai-agent"
}

variable "google_client_id" {
  type        = string
  description = "Google OAuthクライアントID。コールバックURL https://<api_domain>/api/auth/google/callback をGoogle Cloud Console側で許可しておくこと"
  default     = ""
}

variable "google_client_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "github_client_id" {
  type        = string
  description = "GitHub OAuth AppのクライアントID。コールバックURL https://<api_domain>/api/auth/github/callback をGitHub側で許可しておくこと"
  default     = ""
}

variable "github_client_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "domain_name" {
  type        = string
  description = "購入済みドメイン（同名の Route53 ホストゾーンが既に存在すること）"
  default     = "shukatsu-ai.jp"
}

variable "staging_subdomain" {
  type        = string
  description = "staging フロントエンド用サブドメインラベル"
  default     = "stg"
}

variable "staging_api_subdomain" {
  type        = string
  description = "staging バックエンドAPI用サブドメインラベル"
  default     = "api-stg"
}

variable "sentry_dsn" {
  type        = string
  description = "サーバー側(Backend/RAG)のSentry DSN(#1185)。未設定ならSentryは初期化されない。ブラウザ側はビルド時に焼き込むためCIのシークレット(SENTRY_DSN_FRONTEND_STAGING)で渡す"
  sensitive   = true
  default     = ""
}
