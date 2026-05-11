variable "project_name" {
  type = string
}

variable "compartment_id" {
  type        = string
  description = "OCI コンパートメントの OCID"
}

variable "subnet_id" {
  type        = string
  description = "DB配置用プライベートサブネットの OCID"
}

variable "availability_domain" {
  type        = string
  description = "可用性ドメイン名"
}

variable "db_admin_password" {
  type        = string
  sensitive   = true
  description = "MySQL 管理者パスワード (terraform.tfvars で指定)"
}

variable "mysql_version" {
  type    = string
  default = "8.0"
}

variable "shape_name" {
  type    = string
  default = "MySQL.Free"
  description = "Always Free対応: MySQL.Free"
}
