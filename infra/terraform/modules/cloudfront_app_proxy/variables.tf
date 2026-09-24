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

variable "origin_token" {
  type        = string
  sensitive   = true
  description = <<-EOT
    CloudFront がオリジン(ALB)へ必ず付与する経路証明トークン(#1407)。
    ALB は 0.0.0.0/0 に開いているため CloudFront を通さない直接アクセスが成立する。
    frontend(BFF) はこのヘッダーが一致したときだけ X-Forwarded-For の段数を信用し、
    実クライアントIPを Backend へ署名付きで転送する。
    ビューアーが同名ヘッダーを送っても CloudFront が上書きするため詐称できない。
  EOT
}

variable "service_unavailable_html" {
  type        = string
  description = "ALB(500/502/503/504)フェイルオーバー時に返す静的HTML"
}

variable "tags" {
  type    = map(string)
  default = {}
}
