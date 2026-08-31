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
  description = "ALBターゲットグループARN。空文字なら外部公開しない内部サービスとして作成する(load_balancerブロックを付けない)"
  default     = ""
}

variable "service_discovery_registry_arn" {
  type        = string
  description = "Cloud Map(Service Discovery)のサービスARN。空文字なら登録しない"
  default     = ""
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

variable "enable_execute_command" {
  type        = bool
  description = "ECS Exec(デバッグ用シェル接続)を有効にするか"
  default     = false
}

variable "container_health_check" {
  type = object({
    command      = list(string)
    interval     = number
    timeout      = number
    retries      = number
    start_period = number
  })
  description = "メインコンテナのヘルスチェック定義。nullなら付けない"
  default     = null
}

variable "container_mount_points" {
  type        = list(any)
  description = "メインコンテナのmountPoints(ECSのcontainer definition形式)"
  default     = []
}

variable "efs_volumes" {
  type = list(object({
    name            = string
    file_system_id  = string
    file_system_arn = string
    access_point_id = string
  }))
  description = "タスクにマウントするEFSボリューム(永続化が必要なサイドカー、例: chroma用)"
  default     = []
}
