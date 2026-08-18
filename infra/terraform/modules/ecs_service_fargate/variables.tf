variable "project_name" {
  type        = string
  description = "Used for naming log groups / roles"
}

variable "service_name" {
  type = string
}

variable "cluster_id" {
  type = string
}

variable "subnet_ids" {
  type        = list(string)
  description = "タスクを配置するサブネット"
}

variable "security_group_id" {
  type = string
}

variable "assign_public_ip" {
  type        = bool
  description = "NATなしのpublic subnetではECR/CloudWatchへの到達にtrueが必要"
  default     = true
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

variable "cpu" {
  type        = number
  description = "タスクCPUユニット（Fargateの有効な組み合わせに従うこと。例: 256/512/1024）"
  default     = 256
}

variable "memory" {
  type        = number
  description = "タスクメモリMiB（Fargateの有効な組み合わせに従うこと。例: 512/1024/2048）"
  default     = 512
}

variable "desired_count" {
  type    = number
  default = 0
}

variable "target_group_arn" {
  type        = string
  description = "ALBターゲットグループARN"
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

variable "s3_bucket_arn" {
  type        = string
  description = "Optional S3 bucket ARN for task role"
  default     = ""
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

variable "extra_container_definitions" {
  type        = list(any)
  description = "同一タスクに追加するサイドカーコンテナ定義(例: Redis)。ECSのcontainer definition形式をそのまま渡す"
  default     = []
}
