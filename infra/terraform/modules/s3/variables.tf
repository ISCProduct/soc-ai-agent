variable "project_name" {
  type = string
}

variable "force_destroy" {
  type        = bool
  description = "Allow terraform destroy to delete non-empty bucket (staging OK)"
  default     = true
}

variable "tags" {
  type    = map(string)
  default = {}
}
