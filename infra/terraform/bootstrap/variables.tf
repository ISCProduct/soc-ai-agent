variable "region" {
  type        = string
  description = "AWS region"
  default     = "ap-northeast-1"
}

variable "project_name" {
  type        = string
  description = "Name prefix for state backend resources"
  default     = "soc-ai-agent"
}
