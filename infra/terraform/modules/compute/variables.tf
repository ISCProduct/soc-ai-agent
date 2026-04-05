variable "project_name" { type = string }
variable "instance_type" {
  type    = string
  default = "t4g.small"
}
variable "subnet_id" { type = string }
variable "security_group_id" { type = string }
variable "key_name" {
  type    = string
  default = null
}
