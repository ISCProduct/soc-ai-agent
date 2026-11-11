variable "region" {
  type    = string
  default = "ap-northeast-1"
}

variable "project_name" {
  type    = string
  default = "soc-app"
}

variable "vpc_cidr" {
  type    = string
  default = "10.20.0.0/16"
}

variable "azs" {
  type    = list(string)
  default = ["ap-northeast-1a", "ap-northeast-1c"]
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.20.1.0/24", "10.20.2.0/24"]
}

variable "allowed_http_cidrs" {
  type        = list(string)
  description = "ALBの80/443へアクセスを許可するCIDR。可能なら制限すること"
  default     = ["0.0.0.0/0"]
}

# 本番は既定停止（指定起動）方針: docs/architecture/infra-decision-oci-stg-aws-prod.md 参照。
# Fargateはタスク稼働時間分のみ課金されるため、既定 desired_count=0（完全停止）。
# 起動する際は tfvars か -var で 1 以上に上げて apply する。
variable "backend_desired_count" {
  type    = number
  default = 0
}

variable "frontend_desired_count" {
  type    = number
  default = 0
}

variable "rag_review_desired_count" {
  # RAG(履歴書レビュー/ES添削)はbackendの同期依存であり、0だと稼働日でも機能が落ちる。
  # 実際の増減は prod-uptime-scheduler.yml が担う(モジュール側で desired_count は ignore_changes)。
  type    = number
  default = 1
}

variable "backend_cpu" {
  type        = number
  description = "Fargateの有効な組み合わせに従うこと（例: 256/512/1024）"
  default     = 256
}

variable "backend_memory" {
  type    = number
  default = 512
}

variable "frontend_cpu" {
  type    = number
  default = 256
}

variable "frontend_memory" {
  type    = number
  default = 512
}

variable "rag_review_cpu" {
  type    = number
  default = 256
}

variable "rag_review_memory" {
  type    = number
  default = 512
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "db_name" {
  type    = string
  default = "app_db"
}

variable "rds_deletion_protection" {
  type    = bool
  default = true
}

variable "rds_skip_final_snapshot" {
  type    = bool
  default = false
}

variable "rds_backup_retention_period" {
  type    = number
  default = 7
}

variable "backend_image" {
  type        = string
  description = "ECR image URI for backend (tag included)"
}

variable "frontend_image" {
  type        = string
  description = "ECR image URI for frontend (tag included)"
}

variable "rag_review_image" {
  type        = string
  description = "ECR image URI for rag-review (tag included)"
}

variable "openai_secret_arn" {
  type        = string
  description = "既存のSecrets Manager ARNを使う場合(openai_api_key未指定時のフォールバック)"
  default     = ""
}

variable "openai_api_key" {
  type        = string
  description = "OpenAI APIキー(平文)。指定するとTerraform管理のSecrets Manager経由でOPENAI_API_KEYとして注入される"
  sensitive   = true
  default     = ""
}

variable "prompt_version" {
  type        = string
  description = "後方互換の共通プロンプト版ラベル。本文を変更するリリースと同時に更新する"
  default     = "v2"

  # アプリ側の promptVersionPattern と揃える。
  validation {
    condition     = can(regex("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$", var.prompt_version))
    error_message = "prompt_version は英数字で始まり、英数字・.・_・- のみの最大64文字で指定してください。"
  }
}

variable "prompt_versions" {
  type        = map(string)
  description = "工程別プロンプト版ラベル。本文を変更するリリースと同時に更新する"
  default     = {}

  # キーの誤字は環境変数名がずれて黙って無視されるので、4工程に限定する。
  validation {
    condition = alltrue([
      for stage, version in var.prompt_versions :
      contains(["question_generation", "score_evaluation", "matching_reason", "company_extraction"], stage)
      && can(regex("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$", version))
    ])
    error_message = "prompt_versions のキーは question_generation / score_evaluation / matching_reason / company_extraction のいずれかで、値は prompt_version と同じ形式にしてください。"
  }
}

variable "prompt_ab" {
  type        = map(string)
  description = "工程別のプロンプト A/B 割当（例: { score_evaluation = \"v2:50,v3:50\" }）。候補の本文を登録した版だけが有効（#1976）"
  default     = {}

  # キーの誤字は環境変数名がずれて黙って無視されるので、A/B に対応した3工程に限定する。
  validation {
    condition = alltrue([
      for stage, arms in var.prompt_ab :
      contains(["score_evaluation", "matching_reason", "company_extraction"], stage)
      && can(regex("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}:[0-9]{1,5}(,[A-Za-z0-9][A-Za-z0-9._-]{0,63}:[0-9]{1,5})+$", replace(arms, " ", "")))
    ])
    error_message = "prompt_ab のキーは score_evaluation / matching_reason / company_extraction のいずれかで、値は「版:比率」を2つ以上カンマでつないだ形式にしてください。"
  }
}

variable "google_client_id" {
  type        = string
  description = "Google OAuthクライアントID。コールバックURL https://api.<domain_name>/api/auth/google/callback をGoogle Cloud Console側で許可しておくこと"
  default     = ""
}

variable "google_client_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "github_client_id" {
  type        = string
  description = "GitHub OAuth AppのクライアントID。コールバックURL https://api.<domain_name>/api/auth/github/callback をGitHub側で許可しておくこと"
  default     = ""
}

variable "github_client_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "resend_api_key" {
  type        = string
  description = "Resend(メール送信)のAPIキー(#756)"
  sensitive   = true
  default     = ""
}

variable "admin_secret" {
  type        = string
  description = "管理者認証シークレット。未指定なら本番専用の値を自動生成する(#1158)。stagingと同じ値を入れないこと"
  sensitive   = true
  default     = ""
}

variable "secret_values_managed_outside" {
  type        = bool
  description = "外部サービス由来のキー(OpenAI/Resend/OAuth)の値をSecrets Manager側で直接管理している場合はtrue。tfvarsに平文を置かずにplan/applyできるようにする(#1158)。このフラグが効くのは初期構築時のprecondition検査だけで、値の更新はfalseでもTerraformからは行えない(ignore_changesは常時有効で、tfvarsの値が入るのは初回作成時のみ)"
  default     = false
}

variable "additional_secret_arns" {
  type        = list(string)
  description = "Extra secret ARNs the backend execution role may read"
  default     = []
}

variable "frontend_api_base_url" {
  type        = string
  description = "NEXT_PUBLIC_API_BASE_URL。未指定なら https://api.<domain_name> を使用"
  default     = ""
}

variable "domain_name" {
  type        = string
  description = "購入済みドメイン（同名の Route53 ホストゾーンが既に存在すること）"
  default     = "shukatsu-ai.jp"
}

variable "enable_error_fallback" {
  type        = bool
  description = "frontendを常時CloudFront経由にし、ALBが500/502/503/504を返す場合(本番停止中を含む)にS3のOGP付き静的ページへフェイルオーバーするか"
  # 本番(shukatsu-ai.jp / *.shukatsu-ai.jp)は既にCloudFront経由で配信済みのため既定はtrue。
  # falseにするとcloudfront_app_proxyモジュールがcount=0になり、planが本番の
  # CloudFrontディストリビューション・ACM証明書・S3エラーページを破壊対象に含める。
  # 実際に「IAM権限不足の回避」でfalseへ倒したまま放置され、applyできない状態が続いた。
  default = true
}

# AI面接の音声認識モデル。既定はコード側と同じ gpt-transcribe。
# 実音声の誤り率は mini 23% / gpt-transcribe 18%。費用を優先するなら
# gpt-4o-mini-transcribe に戻せる（問題発話だけ gpt-transcribe へ再送される）。
variable "openai_whisper_model" {
  description = "AI面接のSTTモデル。費用を下げるなら gpt-4o-mini-transcribe に変更する"
  type        = string
  default     = "gpt-transcribe"
}

variable "gbizinfo_base_url" {
  type        = string
  description = "gBizINFO Web-APIのベースURL(#1360)。シークレットではないため環境変数として注入する"
  default     = "https://info.gbiz.go.jp/hojin"
}

variable "alarm_email" {
  type        = string
  description = "CloudWatchアラームの通知先メールアドレス。空ならアラームは状態を持つだけで誰にも届かない。購読はメール側でConfirmするまで有効にならない"
  default     = ""
}

variable "sentry_dsn" {
  type        = string
  description = "サーバー側(Backend)のSentry DSN。本番だけ未設定でエラーが記録されていなかった。空ならSentryは初期化されない"
  default     = ""
  sensitive   = true
}

# OpenAI 障害時の音声面接の縮退（#1833）。空なら縮退しない（今まで通り OpenAI だけ）。
# ローカルの推論先は #1832 で用意する。詳細は docs/wiki/ai-failover.md
variable "ai_failover_local_stt_url" {
  type        = string
  description = "Whisper.cpp の server の URL（例 http://10.0.0.5:8080）。空なら STT は縮退しない"
  default     = ""
}

variable "ai_failover_local_llm_url" {
  type        = string
  description = "OpenAI 互換の LLM（Ollama）の URL（例 http://10.0.0.5:11434/v1）。空なら LLM は縮退しない"
  default     = ""
}

variable "ai_failover_local_llm_model" {
  type        = string
  description = "ローカルの LLM のモデル。空ならアプリの既定（qwen3:1.7b）"
  default     = ""
}

variable "ai_failover_local_tts_url" {
  type        = string
  description = "VOICEVOX エンジンの URL（例 http://10.0.0.5:50021）。空なら TTS は縮退しない"
  default     = ""
}

variable "ai_failover_local_tts_speaker" {
  type        = string
  description = "VOICEVOX の話者 ID。空ならアプリの既定（1）"
  default     = ""
}
