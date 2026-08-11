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

variable "instance_type" {
  type    = string
  default = "t4g.small"
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

variable "image_tag" {
  type        = string
  description = "ECR に push するタグ（backend_image/frontend_image 未指定時）"
  default     = "staging"
}

variable "ecr_repository_names" {
  type    = list(string)
  default = ["soc-backend", "soc-frontend"]
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
