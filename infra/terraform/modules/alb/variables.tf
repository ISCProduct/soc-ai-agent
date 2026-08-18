variable "project_name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  type        = list(string)
  description = "ALB用サブネット（異なるAZで2つ以上必要）"
}

variable "security_group_id" {
  type = string
}

variable "route53_zone_id" {
  type        = string
  description = "証明書DNS検証レコードを作成するホストゾーンID"
}

variable "frontend_domain_name" {
  type        = string
  description = "frontend用ドメイン名（例: shukatsu-ai.jp / stg.shukatsu-ai.jp）"
}

variable "backend_domain_name" {
  type        = string
  description = "backend用ドメイン名（例: api.shukatsu-ai.jp / api-stg.shukatsu-ai.jp）"
}

variable "frontend_target_port" {
  type    = number
  default = 3000
}

variable "backend_target_port" {
  type    = number
  default = 8080
}

variable "target_type" {
  type        = string
  description = "\"ip\"（Fargate/awsvpc）または \"instance\"（ECS on EC2/bridge, ホストポート動的割当）"
  default     = "ip"

  validation {
    condition     = contains(["ip", "instance"], var.target_type)
    error_message = "target_type は ip または instance のいずれか。"
  }
}

variable "health_check_path" {
  type        = string
  description = "backend用ヘルスチェックパス（/healthz を実装済み）"
  default     = "/healthz"
}

variable "frontend_health_check_path" {
  type        = string
  description = "frontend用ヘルスチェックパス（Next.jsは/api/healthzにのみ実装、/healthzは404になるため別変数）"
  default     = "/api/healthz"
}

variable "additional_san_domains" {
  type        = list(string)
  description = "ACM証明書に追加するSAN(例: 学校別サブドメイン用の \"*.shukatsu-ai.jp\")"
  default     = []
}

variable "tags" {
  type    = map(string)
  default = {}
}
