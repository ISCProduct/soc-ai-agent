terraform {
  required_providers {
    aws = {
      source                = "hashicorp/aws"
      configuration_aliases = [aws.us_east_1]
    }
  }
}

# ALB が 503 を返すとき（ターゲット全滅）に Route53 フェイルオーバーで配信する静的エラーページ

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
  name                              = "${var.project_name}-errors-${var.env}"
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
      values   = [aws_cloudfront_distribution.errors.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "errors" {
  bucket = aws_s3_bucket.errors.id
  policy = data.aws_iam_policy_document.errors_bucket.json
}

resource "aws_acm_certificate" "errors" {
  provider                  = aws.us_east_1
  domain_name               = var.domain_name
  validation_method         = "DNS"
  subject_alternative_names = var.subject_alternative_names

  lifecycle {
    create_before_destroy = true
  }

  tags = var.tags
}

resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.errors.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      type   = dvo.resource_record_type
      record = dvo.resource_record_value
    }
  }

  zone_id         = var.route53_zone_id
  name            = each.value.name
  type            = each.value.type
  ttl             = 60
  records         = [each.value.record]
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "errors" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.errors.arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

resource "aws_cloudfront_distribution" "errors" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.project_name} error fallback (${var.env})"
  default_root_object = "service-unavailable.html"
  aliases             = var.aliases

  origin {
    domain_name              = aws_s3_bucket.errors.bucket_regional_domain_name
    origin_id                = "s3-errors"
    origin_access_control_id = aws_cloudfront_origin_access_control.errors.id
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-errors"
    viewer_protocol_policy = "redirect-to-https"
    compress               = true

    forwarded_values {
      query_string = false
      cookies { forward = "none" }
    }

    min_ttl     = 0
    default_ttl = 60
    max_ttl     = 300
  }

  # 存在しないパス（/interview 等）も同じ HTML を返す
  custom_error_response {
    error_code            = 403
    response_code         = 503
    response_page_path    = "/service-unavailable.html"
    error_caching_min_ttl = 10
  }

  custom_error_response {
    error_code            = 404
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
    acm_certificate_arn      = aws_acm_certificate_validation.errors.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  tags = var.tags
}
