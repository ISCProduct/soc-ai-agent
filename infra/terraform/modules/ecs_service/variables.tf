variable "project_name" {
  type        = string
  description = "Used for naming log groups / roles when not overridden"
}

variable "service_name" {
  type = string
}

variable "cluster_id" {
  type = string
}

variable "capacity_provider_name" {
  type = string
}

variable "container_name" {
  type = string
}

variable "container_image" {
  type = string
}

variable "container_port" {
  type = number
}

variable "host_port" {
  type        = number
  description = "target_group_arn 未指定時のみ使用（固定ポート直公開）。ALB使用時は動的割当のため無視される"
  default     = 0
}

variable "target_group_arn" {
  type        = string
  description = "指定するとALBターゲットグループに登録し、hostPortは動的割当になる"
  default     = ""
}

variable "cpu" {
  type        = number
  description = "Task CPU units (EC2)"
  default     = 256
}

variable "memory" {
  type        = number
  description = "Task memory MiB (EC2)"
  default     = 512
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "environment" {
  type    = map(string)
  default = {}
}

variable "secrets" {
  type = list(object({
    name      = string
    valueFrom = string
  }))
  default = []
}

variable "secret_arns" {
  type        = list(string)
  description = "Secrets Manager ARNs the execution role may read"
  default     = []
}

# count/for_eachの判定に使う。secret_arns自体が同一apply内で作成される
# リソース(例: Secrets Managerシークレット)を含むと値がapply後にしか定まらず、
# length(var.secret_arns) > 0 のようなcount式がplan時に評価不能になるため、
# 呼び出し側が静的に分かっている真偽値として明示的に渡す。
variable "create_secrets_policy" {
  type        = bool
  description = "secret_arnsを読むIAMポリシーを作成するか(secret_arnsの中身が同一apply内で未確定でも安全に判定できるよう明示的に指定する)"
  default     = false
}

variable "s3_bucket_arn" {
  type        = string
  description = "Optional S3 bucket ARN for task role"
  default     = ""
}

variable "create_s3_policy" {
  type        = bool
  description = "s3_bucket_arnへのIAMポリシーを作成するか(create_secrets_policyと同様の理由で明示指定)"
  default     = false
}

variable "log_retention_days" {
  type    = number
  default = 14
}

variable "region" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}
