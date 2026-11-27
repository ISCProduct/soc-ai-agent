# Discord の受け口を本番 ALB に置く（SOCAIAGENT-512）。
#
# 既定では何も作らない。今の受け口は staging の Lambda で、本番 ECS が
# 止まっていても /prod を受けられるようにしてある。ここを有効にすると
# 本番側にも同じ受け口ができる。朝のリマインドは別フラグで、staging と
# 両方 ENABLED にすると同じ通知が2回投稿される。
#
# 実体のバイナリは CI が update-function-code で入れる。関数が無い間は
# そのジョブは何もしない。

data "aws_caller_identity" "current" {}

resource "aws_cloudwatch_log_group" "discord_lambda" {
  count             = var.discord_bot_enabled ? 1 : 0
  name              = "/aws/lambda/${var.project_name}-discord"
  retention_in_days = 14
  tags              = local.tags
}

resource "aws_iam_role" "discord_lambda" {
  count = var.discord_bot_enabled ? 1 : 0
  name  = "${var.project_name}-discord-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy" "discord_lambda" {
  count = var.discord_bot_enabled ? 1 : 0
  name  = "${var.project_name}-discord-lambda-policy"
  role  = aws_iam_role.discord_lambda[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = ["${aws_cloudwatch_log_group.discord_lambda[0].arn}:*"]
      },
      {
        Sid    = "UptimeParameters"
        Effect = "Allow"
        Action = ["ssm:GetParameter", "ssm:PutParameter"]
        Resource = [
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/prod-uptime-dates",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/prod-uptime-override",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/staging-uptime",
        ]
      },
      {
        Sid      = "LinkParameter"
        Effect   = "Allow"
        Action   = ["ssm:GetParameter", "ssm:PutParameter"]
        Resource = ["arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/discord-backlog-links"]
      },
    ]
  })
}

data "archive_file" "discord_lambda_placeholder" {
  count       = var.discord_bot_enabled ? 1 : 0
  type        = "zip"
  output_path = "${path.module}/.terraform/discord-lambda-placeholder.zip"

  source {
    content  = "placeholder: CI が update-function-code で実体を入れる"
    filename = "bootstrap"
  }
}

resource "aws_lambda_function" "discord" {
  count         = var.discord_bot_enabled ? 1 : 0
  function_name = "${var.project_name}-discord"
  role          = aws_iam_role.discord_lambda[0].arn

  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"

  filename         = data.archive_file.discord_lambda_placeholder[0].output_path
  source_code_hash = data.archive_file.discord_lambda_placeholder[0].output_base64sha256

  timeout     = 10
  memory_size = 128

  environment {
    variables = {
      DISCORD_PUBLIC_KEY           = var.discord_public_key
      DISCORD_ALLOWED_ROLE_ID      = var.discord_allowed_role_id
      GITHUB_DISPATCH_TOKEN        = var.github_dispatch_token
      GITHUB_DISPATCH_REPO         = var.github_dispatch_repo
      BACKLOG_API_KEY              = var.backlog_api_key
      DISCORD_REMINDER_WEBHOOK_URL = var.discord_reminder_webhook_url
      BACKLOG_REVIEWER_IDS         = var.backlog_reviewer_ids
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.discord_lambda[0].name
  }

  lifecycle {
    ignore_changes = [filename, source_code_hash]
  }

  depends_on = [aws_iam_role_policy.discord_lambda]

  tags = local.tags
}

resource "aws_lb_target_group" "discord" {
  count       = var.discord_bot_enabled ? 1 : 0
  name        = "${var.project_name}-discord-tg"
  target_type = "lambda"
  tags        = local.tags
}

resource "aws_lambda_permission" "alb" {
  count         = var.discord_bot_enabled ? 1 : 0
  statement_id  = "AllowExecutionFromALB"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.discord[0].arn
  principal     = "elasticloadbalancing.amazonaws.com"
  source_arn    = aws_lb_target_group.discord[0].arn
}

resource "aws_lb_target_group_attachment" "discord" {
  count            = var.discord_bot_enabled ? 1 : 0
  target_group_arn = aws_lb_target_group.discord[0].arn
  target_id        = aws_lambda_function.discord[0].arn
  depends_on       = [aws_lambda_permission.alb]
}

resource "aws_lb_listener_rule" "discord" {
  count        = var.discord_bot_enabled ? 1 : 0
  listener_arn = module.alb.https_listener_arn
  priority     = 90

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.discord[0].arn
  }

  condition {
    host_header {
      values = [local.backend_domain]
    }
  }

  condition {
    path_pattern {
      values = ["/api/discord/interactions"]
    }
  }

  tags = local.tags
}

resource "aws_iam_role" "discord_scheduler" {
  count = var.discord_bot_enabled ? 1 : 0
  name  = "${var.project_name}-discord-scheduler"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "scheduler.amazonaws.com"
      }
      Action = "sts:AssumeRole"
      Condition = {
        StringEquals = {
          "aws:SourceAccount" = data.aws_caller_identity.current.account_id
        }
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy" "discord_scheduler" {
  count = var.discord_bot_enabled ? 1 : 0
  name  = "${var.project_name}-discord-scheduler"
  role  = aws_iam_role.discord_scheduler[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "lambda:InvokeFunction"
      Resource = aws_lambda_function.discord[0].arn
    }]
  })
}

resource "aws_scheduler_schedule" "backlog_deadline_reminder" {
  count       = var.discord_bot_enabled ? 1 : 0
  name        = "${var.project_name}-backlog-deadline-reminder"
  description = "Backlog の期限が近い・切れた課題を担当者へ Discord でリマインドする(平日 9:30 JST)"
  state       = var.discord_reminder_schedule_enabled ? "ENABLED" : "DISABLED"

  schedule_expression          = "cron(30 9 ? * MON-FRI *)"
  schedule_expression_timezone = "Asia/Tokyo"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.discord[0].arn
    role_arn = aws_iam_role.discord_scheduler[0].arn
    input = jsonencode({
      source = "backlog-deadline-reminder"
    })

    retry_policy {
      maximum_retry_attempts       = 0
      maximum_event_age_in_seconds = 60
    }
  }
}

resource "aws_cloudwatch_metric_alarm" "discord_lambda_errors" {
  count               = var.discord_bot_enabled ? 1 : 0
  alarm_name          = "${var.project_name}-discord-lambda-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
  alarm_description   = "Discord Lambda の失敗。朝のリマインドは再試行しないので、この通知が障害の知らせになる"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  ok_actions          = [aws_sns_topic.alarms.arn]

  dimensions = {
    FunctionName = aws_lambda_function.discord[0].function_name
  }

  tags = local.tags
}

output "discord_interactions_endpoint" {
  description = "本番の Discord Interactions Endpoint。discord_bot_enabled のあとで Portal に設定する"
  value       = var.discord_bot_enabled ? "https://${local.backend_domain}/api/discord/interactions" : null
}
