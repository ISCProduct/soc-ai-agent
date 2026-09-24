# アクセスログ用バケット。有効化したときだけ作る。
#
# 2026-09 の調査で、14日間に ELB 5xx が 50,997 件発生していたのに
# 発生元をまったく特定できなかった。アクセスログが無効で、Sentry も本番だけ
# 未設定、CloudWatch アラームも0件という状態だったため、誰も気付いていなかった。
# 障害の切り分け以前に「誰が来ているのか」が分からない。
resource "aws_s3_bucket" "access_logs" {
  count         = var.enable_access_logs ? 1 : 0
  bucket        = "${var.project_name}-alb-logs"
  force_destroy = false

  tags = merge(var.tags, {
    Name = "${var.project_name}-alb-logs"
  })
}

resource "aws_s3_bucket_public_access_block" "access_logs" {
  count  = var.enable_access_logs ? 1 : 0
  bucket = aws_s3_bucket.access_logs[0].id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "access_logs" {
  count  = var.enable_access_logs ? 1 : 0
  bucket = aws_s3_bucket.access_logs[0].id

  rule {
    apply_server_side_encryption_by_default {
      # ALB のアクセスログは SSE-KMS を受け付けない（SSE-S3 のみ）。
      sse_algorithm = "AES256"
    }
  }
}

# 放置すると増え続けるだけなので保持期間を切る。
# CloudWatch Logs の保持(14日)より長くしているのは、アクセスログは
# 後追い調査で遡ることが多いため。
resource "aws_s3_bucket_lifecycle_configuration" "access_logs" {
  count  = var.enable_access_logs ? 1 : 0
  bucket = aws_s3_bucket.access_logs[0].id

  rule {
    id     = "expire"
    status = "Enabled"

    filter {}

    expiration {
      days = var.access_logs_retention_days
    }
  }
}

# ALB がログを書き込むための権限。東京リージョンの ELB サービスアカウントを使う。
data "aws_elb_service_account" "this" {
  count = var.enable_access_logs ? 1 : 0
}

data "aws_iam_policy_document" "access_logs" {
  count = var.enable_access_logs ? 1 : 0

  statement {
    principals {
      type        = "AWS"
      identifiers = [data.aws_elb_service_account.this[0].arn]
    }
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.access_logs[0].arn}/*"]
  }
}

resource "aws_s3_bucket_policy" "access_logs" {
  count  = var.enable_access_logs ? 1 : 0
  bucket = aws_s3_bucket.access_logs[0].id
  policy = data.aws_iam_policy_document.access_logs[0].json
}

resource "aws_lb" "this" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  subnets            = var.subnet_ids
  security_groups    = [var.security_group_id]

  dynamic "access_logs" {
    for_each = var.enable_access_logs ? [1] : []
    content {
      bucket  = aws_s3_bucket.access_logs[0].id
      enabled = true
    }
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-alb"
  })

  depends_on = [aws_s3_bucket_policy.access_logs]
}

resource "aws_lb_target_group" "frontend" {
  name                 = "${var.project_name}-fe-tg"
  port                 = var.frontend_target_port
  protocol             = "HTTP"
  vpc_id               = var.vpc_id
  target_type          = var.target_type
  deregistration_delay = var.deregistration_delay

  health_check {
    path                = var.frontend_health_check_path
    matcher             = "200"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    # 30秒間隔だと healthy 判定まで最短60秒かかり、デプロイの待ち時間に直接乗る。
    # 10秒へ短縮して最短20秒にする（/healthz は軽量なので負荷増は無視できる）。
    interval = 10
    timeout  = 5
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-fe-tg"
  })
}

resource "aws_lb_target_group" "backend" {
  name                 = "${var.project_name}-be-tg"
  port                 = var.backend_target_port
  protocol             = "HTTP"
  vpc_id               = var.vpc_id
  target_type          = var.target_type
  deregistration_delay = var.deregistration_delay

  health_check {
    path                = var.health_check_path
    matcher             = "200"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    # 30秒間隔だと healthy 判定まで最短60秒かかり、デプロイの待ち時間に直接乗る。
    # 10秒へ短縮して最短20秒にする（/healthz は軽量なので負荷増は無視できる）。
    interval = 10
    timeout  = 5
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-be-tg"
  })
}

# --- ACM証明書（DNS検証） ---

resource "aws_acm_certificate" "this" {
  domain_name               = var.frontend_domain_name
  subject_alternative_names = concat([var.backend_domain_name], var.additional_san_domains)
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = merge(var.tags, {
    Name = "${var.project_name}-cert"
  })
}

resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.this.domain_validation_options : dvo.domain_name => {
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

resource "aws_acm_certificate_validation" "this" {
  certificate_arn         = aws_acm_certificate.this.arn
  validation_record_fqdns = [for r in aws_route53_record.cert_validation : r.fqdn]
}

# --- リスナー ---

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.this.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.frontend.arn
  }
}

resource "aws_lb_listener_rule" "backend" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 100

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.backend.arn
  }

  condition {
    host_header {
      values = [var.backend_domain_name]
    }
  }
}
