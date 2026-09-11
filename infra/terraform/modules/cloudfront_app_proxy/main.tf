terraform {
  required_providers {
    aws = {
      source                = "hashicorp/aws"
      configuration_aliases = [aws.us_east_1]
    }
  }
}

# CloudFrontを常時ALBの手前に置き、ALBが500/502/503/504を返した場合(意図的な
# 停止desired_count=0を含む)にS3の静的ページへフェイルオーバーする。
# ALB配下は動的アプリ(Cookie認証・OAuthコールバック・SSEストリーミング)のため、
# キャッシュは無効化しビューアーリクエストを丸ごとオリジンへ転送する。

resource "aws_s3_bucket" "errors" {
  bucket        = "${var.project_name}-errors-${var.env}"
  force_destroy = true

  tags = merge(var.tags, {
    Name = "${var.project_name}-errors-${var.env}"
  })
}

resource "aws_s3_bucket_public_access_block" "errors" {
  bucket = aws_s3_bucket.errors.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "errors" {
  bucket = aws_s3_bucket.errors.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_object" "service_unavailable" {
  bucket       = aws_s3_bucket.errors.id
  key          = "service-unavailable.html"
  content      = var.service_unavailable_html
  content_type = "text/html; charset=utf-8"
  etag         = md5(var.service_unavailable_html)
}

resource "aws_cloudfront_origin_access_control" "errors" {
  name                              = "${var.project_name}-app-${var.env}"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

data "aws_iam_policy_document" "errors_bucket" {
  statement {
    sid    = "AllowCloudFrontRead"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.errors.arn}/*"]

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.app.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "errors" {
  bucket = aws_s3_bucket.errors.id
  policy = data.aws_iam_policy_document.errors_bucket.json
}

resource "aws_acm_certificate" "app" {
  provider                  = aws.us_east_1
  domain_name               = var.domain_name
  subject_alternative_names = var.subject_alternative_names
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = var.tags
}

resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.app.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      type   = dvo.resource_record_type
      record = dvo.resource_record_value
    }
  }

  zone_id         = var.route53_zone_id
  name            = each.value.name
  type            = each.value.type
  ttl             = 300
  records         = [each.value.record]
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "app" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.app.arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

# ALB配下は動的コンテンツ(認証Cookie等)のためキャッシュを無効化する
resource "aws_cloudfront_cache_policy" "no_cache" {
  name = "${var.project_name}-${var.env}-app-no-cache"

  min_ttl     = 0
  default_ttl = 0
  max_ttl     = 0

  # キャッシュ完全無効(TTL=0)のため、Cookie/クエリのforward先はここでなく
  # 下記 origin_request_policy(allViewer)側が担う。CloudFront API仕様上、
  # キャッシュ無効時はcookie/query_stringのbehaviorをnone以外にできない。
  parameters_in_cache_key_and_forwarded_to_origin {
    cookies_config {
      cookie_behavior = "none"
    }
    headers_config {
      header_behavior = "none"
    }
    query_strings_config {
      query_string_behavior = "none"
    }
    enable_accept_encoding_gzip   = false
    enable_accept_encoding_brotli = false
  }
}

# ALBはHost headerでfrontend/backend/学園サブドメインをルーティングするため、
# Hostを含む全ビューアーヘッダーをそのままオリジンへ転送する。
resource "aws_cloudfront_origin_request_policy" "all_viewer" {
  name = "${var.project_name}-${var.env}-app-all-viewer"

  cookies_config {
    cookie_behavior = "all"
  }
  headers_config {
    header_behavior = "allViewer"
  }
  query_strings_config {
    query_string_behavior = "all"
  }
}

resource "aws_cloudfront_distribution" "app" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.project_name} app proxy (${var.env})"
  default_root_object = ""
  aliases             = var.aliases

  origin {
    domain_name = var.alb_dns_name
    origin_id   = "alb"

    custom_origin_config {
      http_port                = 80
      https_port               = 443
      origin_protocol_policy   = "https-only"
      origin_ssl_protocols     = ["TLSv1.2"]
      origin_keepalive_timeout = 5
      origin_read_timeout      = 60
    }
  }

  origin {
    domain_name              = aws_s3_bucket.errors.bucket_regional_domain_name
    origin_id                = "s3-errors"
    origin_access_control_id = aws_cloudfront_origin_access_control.errors.id
  }

  # origin_group(自動フェイルオーバー)はPOST/PUT/PATCH/DELETEを含む
  # キャッシュビヘイビアで使えない(CloudFront側の制約)ため、ALBを
  # 単一オリジンとし、500/502/503/504はcustom_error_responseで
  # 静的ページ(別ビヘイビア経由でS3から直接取得)に差し替える。
  default_cache_behavior {
    allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "PATCH", "POST", "DELETE"]
    cached_methods           = ["GET", "HEAD"]
    target_origin_id         = "alb"
    viewer_protocol_policy   = "redirect-to-https"
    compress                 = true
    cache_policy_id          = aws_cloudfront_cache_policy.no_cache.id
    origin_request_policy_id = aws_cloudfront_origin_request_policy.all_viewer.id
  }

  # 静的ページ自体はS3から直接取得(GET/HEADのみのため制約なし)
  ordered_cache_behavior {
    path_pattern             = "/service-unavailable.html"
    allowed_methods          = ["GET", "HEAD"]
    cached_methods           = ["GET", "HEAD"]
    target_origin_id         = "s3-errors"
    viewer_protocol_policy   = "redirect-to-https"
    compress                 = true
    cache_policy_id          = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # AWS managed: CachingDisabled
  }

  custom_error_response {
    error_code            = 500
    response_code         = 503
    response_page_path    = "/service-unavailable.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 502
    response_code         = 503
    response_page_path    = "/service-unavailable.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 503
    response_code         = 503
    response_page_path    = "/service-unavailable.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 504
    response_code         = 503
    response_page_path    = "/service-unavailable.html"
    error_caching_min_ttl = 10
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate_validation.app.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  tags = var.tags
}
