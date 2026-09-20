# 本番の起動判定を AWS 側のスケジューラで発火させる（#1388）。
#
# prod-uptime-scheduler.yml は cron: "5 * * * *" で毎時動く前提だが、
# GitHub ホストの scheduled workflow はベストエフォートで発火が保証されない。
# 実測（2026-09-18〜09-20 の19回）では
#
#   最小 125分 / 中央 237分 / 最大 334分 / 60分以内の発火 0%
#
# となっており、稼働日（展示会当日）の朝に本番が起動していないリスクがある。
# #1355 の失敗通知は「ジョブが動いて失敗した」ときに鳴るもので、
# そもそも発火しなければ鳴らない。
#
# ここでは発火だけを AWS に任せ、起動手順そのもの（RDS待ち、
# chroma→rag-review→backend→frontend の順序、オートスケーリング下限の同期）は
# 既存のワークフローに残す。Go や Terraform に書き直すと二重管理になり、
# 片方だけ直して本番が中途半端に起動する事故につながる。
#
# GitHub 側の cron は残す。どちらかが動けば反映されるため、
# 二重に走っても desired_count を同じ値に揃えるだけで害が無い。

resource "aws_iam_role" "prod_uptime_scheduler" {
  name = "${var.project_name}-prod-uptime-scheduler"

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

resource "aws_iam_role_policy" "prod_uptime_scheduler" {
  name = "${var.project_name}-prod-uptime-scheduler"
  role = aws_iam_role.prod_uptime_scheduler.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "lambda:InvokeFunction"
      Resource = aws_lambda_function.discord.arn
    }]
  })
}

resource "aws_scheduler_schedule" "prod_uptime" {
  name        = "${var.project_name}-prod-uptime"
  description = "本番の起動判定ワークフローを毎時起動する（GitHub cron の発火が保証されないため / #1388）"

  # 毎時5分(UTC)。既存ワークフローの cron と同じタイミングに合わせる。
  schedule_expression          = "cron(5 * * * ? *)"
  schedule_expression_timezone = "UTC"

  # 発火が遅れても取り消さず必ず実行する。
  # 稼働日の起動が目的なので、多少遅れても実行されるほうがよい。
  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.discord.arn
    role_arn = aws_iam_role.prod_uptime_scheduler.arn

    # source は Lambda 側が Discord の Interaction と区別するための目印。
    # workflow を空にすると PROD_UPTIME_WORKFLOW_FILE の既定が使われる。
    input = jsonencode({
      source   = "prod-uptime-scheduler"
      workflow = "prod-uptime-scheduler.yml"
    })

    retry_policy {
      # 一時的な GitHub API の失敗を拾う。1時間後には次の発火が来るので
      # 長く粘る必要はない。
      maximum_retry_attempts       = 3
      maximum_event_age_in_seconds = 600
    }
  }
}
