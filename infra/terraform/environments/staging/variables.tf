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

variable "prompt_version" {
  type        = string
  description = "後方互換の共通プロンプト版ラベル。本文を変更するリリースと同時に更新する"
  default     = "v2"

  # アプリ側の promptVersionPattern と揃える。値は .env へそのまま書かれるので、
  # 改行などで別の環境変数行を注入できないようにする。
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

variable "secret_values_managed_outside" {
  type        = bool
  description = "シークレットの値を Secrets Manager 側で直接管理している場合は true。tfvarsに平文を置かずにplan/applyできるようにする(#1158)。このフラグが効くのは初回構築時のprecondition検査だけで、値の更新はfalseでもTerraformからは行えない(ignore_changesは常時有効で、tfvarsの値が入るのは初回作成時のみ)"
  default     = false
}

variable "openai_api_key_plain" {
  type        = string
  description = "OpenAI APIキー。初回 apply で Secrets Manager <project>/app-env の openai_api_key へ書き込み、以降は Secrets Manager 側が正(#1158)。未設定でも apply 可"
  sensitive   = true
  default     = ""
}

variable "gbizinfo_api_key" {
  type        = string
  description = "gBizINFO Web-APIのアクセストークン。初回 apply で Secrets Manager <project>/app-env へ書き込む(#1158)。未設定時はgBizINFO連携が無効になり、企業情報の取得がweb_searchに寄る"
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
  description = "国税庁 法人番号システムWeb-APIのアプリケーションID。初回 apply で Secrets Manager <project>/app-env へ書き込む(#1158)。未設定時は法人番号の自動特定だけが無効になる"
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
  description = "Resend APIキー。初回 apply で Secrets Manager <project>/app-env へ書き込む(#1158)。未設定時はEMAIL_PROVIDERがlogにフォールバックする"
  sensitive   = true
  default     = ""
}

variable "admin_secret_plain" {
  type        = string
  description = "管理者トークン(X-Admin-Token)のHMAC鍵。CIには渡さない(#1158: CIが叩く共有シークレットは service_secret_plain)。初回 apply で Secrets Manager <project>/app-env へ書き込み、以降は Secrets Manager 側が正"
  sensitive   = true
  default     = ""
}

variable "service_secret_plain" {
  type        = string
  description = "CIからのサービス間呼び出し(X-Admin-Secret)用の共有シークレット。GitHubのリポジトリシークレット SERVICE_SECRET と同じ値にする。admin_secret_plain とは必ず別値にする(#1158)。初回 apply で Secrets Manager <project>/app-env へ書き込み、以降は Secrets Manager 側が正"
  sensitive   = true

  # 既定値を持たせない。空のまま apply すると SERVICE_SECRET も aws_ssm_parameter も
  # 空になり、CI の取り込みが 503 で拒否される（CI 側は警告で流すので気づきにくい）。
  # plan の時点で止めて、tfvars への設定漏れをその場で分かるようにする。
  #
  # 条件は「禁止したい文字を1つずつ足す」形ではなく許可リスト1本にしている。
  # この値は .env の1行として書かれ、Compose がそれを読むため、壊れ方が何通りもある:
  #   - `$`      : Compose が引用なしの値を変数展開する（未定義名は空文字になる）
  #   - `#`      : Compose が ` #` 以降をインラインコメントとして落とす
  #   - 改行     : 2行目以降が別の環境変数として読まれる（引用では防げない）
  #   - 空白/タブ: 値の途中で切れる・詰められる
  #   - "None"   : AWS CLI が null を None と出すため、デプロイ側の
  #                `[ "$X" != None ]` 判定が「未取得」と誤認し、稼働中インスタンスの
  #                .env を更新しない。新規インスタンスは user_data 経由で None を
  #                受け取るので、staging の値が新旧インスタンスで二つに分かれる
  # 1つずつ禁止していくと、次に別の文字で同じ穴が開く。文字種と最小長で一度に閉じる。
  #
  # 文字種は本番の random_password(special = false → 英数字のみ)と、
  # terraform.tfvars.example が案内している生成方法
  # (`python3 -c "import secrets; print(secrets.token_urlsafe(36))"` → 英数字 + `_` `-`)
  # の両方を通す範囲。最小長32は token_urlsafe(24) の長さで、本番の 48 より緩い下限。
  # 上の壊れ方はすべてこの1本で落ちる（"None" は長さと文字種の両方で落ちる）。
  validation {
    condition     = can(regex("^[A-Za-z0-9_-]{32,}$", var.service_secret_plain))
    error_message = "service_secret_plain は英数字・アンダースコア・ハイフンのみ32文字以上にしてください。.env の1行として Compose が読むため、$ ・ # ・空白・改行を含む値は壊れます。生成例: python3 -c \"import secrets; print(secrets.token_urlsafe(36))\" (#1158)"
  }

  # 形式は正しいが公開されている値を弾く。上の許可リストとは目的が違うので別に持つ。
  # terraform.tfvars.example をそのままコピーして apply すると、例示値が本物の
  # SERVICE_SECRET として .env と SSM へ入り、リポジトリを読める第三者が
  # /api/admin/whats-new/ingest と /api/admin/training/stats を叩けてしまう。
  # 例示値は Backend/.env.example と同じ change-me-service-secret に揃えてあり
  # (Backend/internal/config/config.go の serviceSecretPlaceholder と同値)、ここで弾く。
  # 対応関係は automation/test/service-secret-placeholder-test.sh が固定している。
  validation {
    condition     = var.service_secret_plain != "change-me-service-secret"
    error_message = "service_secret_plain がプレースホルダーのままです。terraform.tfvars.example の値をコピーしただけだと、リポジトリを読める第三者がサービス間エンドポイントを叩けます(#1158)。"
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

variable "backlog_api_key" {
  type        = string
  sensitive   = true
  description = "Discordの /link で Backlog のメンバー一覧を引くためのAPIキー(読み取りのみ)。未設定なら /link 系は「利用できません」を返す"
  default     = ""
}

variable "discord_reminder_webhook_url" {
  type        = string
  sensitive   = true
  description = "Backlog の期限リマインドを投稿する Discord Webhook URL。未設定なら期限リマインドは動かない(他のコマンドには影響しない)"
  default     = ""
}

variable "backlog_reviewer_ids" {
  type        = string
  description = "PR確認待ちの課題でレビューを促す Backlog ユーザーID(カンマ区切り)。空なら促さない"
  default     = "2505302"
}

variable "github_dispatch_repo" {
  type        = string
  description = "workflow_dispatch の対象リポジトリ(owner/repo)"
  default     = "ISCProduct/soc-ai-agent"
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
