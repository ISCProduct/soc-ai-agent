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

variable "openai_secret_arn" {
  type        = string
  description = "Secrets Manager ARN for OPENAI_API_KEY"
  default     = ""
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
