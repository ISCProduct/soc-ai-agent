variable "project_name" {
  type = string
}

variable "env" {
  type = string
}

variable "domain_name" {
  type        = string
  description = "ACM プライマリドメイン（例: stg.shukatsu-ai.jp）"
}

variable "aliases" {
  type        = list(string)
  description = "CloudFront エイリアス"
}

variable "subject_alternative_names" {
  type    = list(string)
  default = []
}

variable "route53_zone_id" {
  type = string
}

variable "service_unavailable_html" {
  type = string
}

variable "tags" {
  type    = map(string)
  default = {}
}
