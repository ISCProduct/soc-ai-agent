variable "region" {
  type    = string
  default = "ap-northeast-1"
}
variable "project_name" {
  type    = string
  default = "soc-app"
}
variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}
variable "allowed_ssh_cidr" {
  type    = list(string)
  default = ["0.0.0.0/0"]
}
variable "instance_type" {
  type    = string
  default = "t4g.small"
}
variable "key_name" {
  type    = string
  default = null
}
variable "domain_name" {
  type        = string
  default     = "it-industryanalysis.jp"
  description = "購入済みのドメイン名 (例: example.com)"
}

variable "secret_arns" {
  type        = list(string)
  description = "EC2 から読み取りを許可する Secrets Manager ARN のリスト。terraform.tfvars で指定すること（git 管理外）"
}
