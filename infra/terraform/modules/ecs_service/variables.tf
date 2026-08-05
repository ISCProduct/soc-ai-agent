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
  type = number
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
