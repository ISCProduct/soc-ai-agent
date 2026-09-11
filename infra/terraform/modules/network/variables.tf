variable "project_name" {
  type        = string
  description = "Name prefix (e.g. soc-stg)"
}

variable "vpc_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

variable "azs" {
  type        = list(string)
  description = "At least 2 AZs required for RDS subnet group"
  default     = ["ap-northeast-1a", "ap-northeast-1c"]
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.10.1.0/24", "10.10.2.0/24"]
}

variable "allowed_http_cidrs" {
  type        = list(string)
  description = "CIDRs allowed to reach HTTP(S) / app ports on ECS hosts"
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

variable "tags" {
  type    = map(string)
  default = {}
}

variable "enable_alb" {
  type        = bool
  description = "true の場合、ALB用SGとFargateタスク用SG（ALBからのみ許可）を追加作成する（prod/Fargate向け）"
  default     = false
}

variable "alb_ingress_cidrs" {
  type        = list(string)
  description = "ALBの80/443へアクセスを許可するCIDR"
  default     = ["0.0.0.0/0"]
}

variable "fargate_container_ports" {
  type        = list(number)
  description = "ALBからFargateタスクへ許可するコンテナポート"
  default     = [3000, 8080]
}
