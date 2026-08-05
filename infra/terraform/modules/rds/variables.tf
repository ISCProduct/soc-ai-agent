variable "project_name" {
  type = string
}

variable "subnet_ids" {
  type        = list(string)
  description = "At least 2 subnets in different AZs"
}

variable "security_group_ids" {
  type = list(string)
}

variable "instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "engine_version" {
  type    = string
  default = "8.0"
}

variable "allocated_storage" {
  type    = number
  default = 20
}

variable "db_name" {
  type    = string
  default = "app_db"
}

variable "master_username" {
  type    = string
  default = "app_user"
}

variable "backup_retention_period" {
  type    = number
  default = 3
}

variable "deletion_protection" {
  type    = bool
  default = false
}

variable "skip_final_snapshot" {
  type    = bool
  default = true
}

variable "tags" {
  type    = map(string)
  default = {}
}
