variable "project_name" {
  type = string
}

variable "env" {
  type = string
}

variable "domain_name" {
  type        = string
  description = "ACM プライマリドメイン（例: shukatsu-ai.jp）"
}

variable "aliases" {
  type        = list(string)
  description = "CloudFront エイリアス（frontend_domainとワイルドカードサブドメイン）"
}

variable "subject_alternative_names" {
  type    = list(string)
  default = []
}

variable "route53_zone_id" {
  type = string
}

variable "alb_dns_name" {
  type        = string
  description = "オリジンとなるALBのDNS名"
}

variable "service_unavailable_html" {
  type        = string
  description = "ALB(500/502/503/504)フェイルオーバー時に返す静的HTML"
}

variable "tags" {
  type    = map(string)
  default = {}
}
