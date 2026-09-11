variable "project_name" { type = string }
variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}
variable "public_subnet_cidr" {
  type    = string
  default = "10.0.1.0/24"
}
variable "az" {
  type    = string
  default = "ap-northeast-1a"
}
variable "allowed_ssh_cidr" {
  type    = list(string)
  default = ["0.0.0.0/0"] # 本来は制限すべき
}
