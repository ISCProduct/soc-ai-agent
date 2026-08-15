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

# --- IAMロールなしのプレーンEC2構成用(main.tfのaws_instance.app参照) ---

variable "aws_access_key_id" {
  type        = string
  description = "EC2にIAMロールを付与できないため、user_dataに埋め込むIAMユーザーのアクセスキー(必ずterraform.tfvars、gitignore対象)"
  sensitive   = true
  default     = ""
}

variable "aws_secret_access_key" {
  type        = string
  description = "上記のシークレットキー"
  sensitive   = true
  default     = ""
}

variable "openai_api_key_plain" {
  type        = string
  description = "OpenAI APIキー(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定でも apply 可"
  sensitive   = true
  default     = ""
}

variable "resend_api_key_plain" {
  type        = string
  description = "Resend APIキー(平文、Secrets Manager未使用のためuser_dataに直接埋め込む)。未設定時はEMAIL_PROVIDERがlogにフォールバックする"
  sensitive   = true
  default     = ""
}

variable "admin_secret_plain" {
  type        = string
  description = "管理者認証シークレット(平文)。CI(sync-whats-newジョブ等)から既知の値で呼び出せるよう、random_passwordではなく固定値にする"
  sensitive   = true
  default     = ""
}

variable "google_client_id" {
  type        = string
  description = "Google OAuthクライアントID。コールバックURL https://<api_domain>/api/auth/google/callback をGoogle Cloud Console側で許可しておくこと"
  default     = ""
}

variable "google_client_secret" {
  type        = string
  sensitive   = true
  default     = ""
}

variable "github_client_id" {
  type        = string
  description = "GitHub OAuth AppのクライアントID。コールバックURL https://<api_domain>/api/auth/github/callback をGitHub側で許可しておくこと"
  default     = ""
}

variable "github_client_secret" {
  type        = string
  sensitive   = true
  default     = ""
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
