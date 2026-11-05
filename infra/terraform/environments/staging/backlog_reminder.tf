# Backlog の期限リマインドを平日朝に発火する。
#
# GitHub Actions の schedule は遅延・欠落するため(進捗リマインダーで実測済み)、
# 発火は AWS のスケジューラに任せる。prod-uptime と同じ Lambda を呼び、
# source で処理を分ける。呼び出しの権限は prod-uptime 用のロールを使い回す
# (どちらも「discord Lambda を InvokeFunction するだけ」のため)。

resource "aws_scheduler_schedule" "backlog_deadline_reminder" {
  name        = "${var.project_name}-backlog-deadline-reminder"
  description = "Backlog の期限が近い・切れた課題を担当者へ Discord でリマインドする(平日 9:30 JST)"

  schedule_expression          = "cron(30 9 ? * MON-FRI *)"
  schedule_expression_timezone = "Asia/Tokyo"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_function.discord.arn
    role_arn = aws_iam_role.prod_uptime_scheduler.arn

    input = jsonencode({
      source = "backlog-deadline-reminder"
    })

    retry_policy {
      # 再試行しない。投稿が成功した後に失敗扱いになって再実行されると、
      # 同じ通知が二重に飛ぶ。失敗は CloudWatch に残る。翌営業日にまた発火する。
      maximum_retry_attempts       = 0
      maximum_event_age_in_seconds = 60
    }
  }
}
