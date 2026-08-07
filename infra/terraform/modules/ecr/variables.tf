variable "repository_names" {
  type        = list(string)
  description = "ECR repository names to create"
  default     = ["soc-backend", "soc-frontend"]
}

variable "image_tag_mutability" {
  type    = string
  default = "MUTABLE"
}

variable "scan_on_push" {
  type    = bool
  default = true
}

variable "force_delete" {
  type        = bool
  description = "Allow destroy even if images remain (staging 向け true 可)"
  default     = false
}

variable "lifecycle_keep_count" {
  type    = number
  default = 20
}

variable "tags" {
  type    = map(string)
  default = {}
}
