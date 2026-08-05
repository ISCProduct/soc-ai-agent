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
  description = "ECR image URI for backend (tag included)"
}

variable "frontend_image" {
  type        = string
  description = "ECR image URI for frontend (tag included)"
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
