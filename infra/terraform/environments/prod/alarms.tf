# 本番の異常を人へ届ける。
#
# 2026-09 の調査時点で CloudWatch アラームは prod / staging とも 0 件だった。
# 14日間に ELB 5xx が 50,997 件出ていても誰にも通知されず、気付かれないまま
# 本番デプロイのマイグレーションも3回連続で失敗していた。
#
# 設計上の要点は「計画停止中に鳴らさない」こと。
# 本番は「指定日のみ終日起動」で、稼働していない時間の方が長い。停止中に
# 鳴るアラームを置くと、通知が無視される状態になり、結局いまと同じになる。
# そのため:
#   - treat_missing_data = "notBreaching"（データが無い＝停止中は正常扱い）
#   - 停止中でも出る ELB 5xx（ターゲット全滅の503）は対象にしない
#   - 稼働していないと発生しない指標だけを見る

resource "aws_sns_topic" "alarms" {
  name = "${var.project_name}-alarms"
  tags = local.tags
}

# メール宛先。設定しないとアラームは状態を持つだけで誰にも届かない。
# 購読はメール側で確認(Confirm)するまで有効にならない。
resource "aws_sns_topic_subscription" "alarms_email" {
  count     = var.alarm_email == "" ? 0 : 1
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.alarm_email
}

locals {
  alb_dimension = {
    LoadBalancer = replace(module.alb.alb_arn, "/^.*:loadbalancer\\//", "")
  }
}

# アプリが返した 5xx。ALB自身が返す503(ターゲット全滅=計画停止)とは別物で、
# こちらは稼働中にしか出ない。1件でも出たら中身を見る必要がある。
resource "aws_cloudwatch_metric_alarm" "target_5xx" {
  alarm_name          = "${var.project_name}-target-5xx"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "HTTPCode_Target_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = 300
  statistic           = "Sum"
  threshold           = 5
  dimensions          = local.alb_dimension
  treat_missing_data  = "notBreaching"
  alarm_description   = "アプリが5xxを返している。計画停止中のELB 5xxとは別で、稼働中のみ発生する"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  ok_actions          = [aws_sns_topic.alarms.arn]
  tags                = local.tags
}

# 応答が遅い。展示会中に体感で分かる前に気付くため。
resource "aws_cloudwatch_metric_alarm" "target_latency" {
  alarm_name          = "${var.project_name}-target-latency"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "TargetResponseTime"
  namespace           = "AWS/ApplicationELB"
  period              = 300
  extended_statistic  = "p95"
  threshold           = 3
  dimensions          = local.alb_dimension
  treat_missing_data  = "notBreaching"
  alarm_description   = "p95応答時間が3秒を超えている"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  tags                = local.tags
}

# ディスク枯渇。RDSは停止中もストレージ課金され、枯渇すると起動しても書けない。
resource "aws_cloudwatch_metric_alarm" "rds_storage" {
  alarm_name          = "${var.project_name}-rds-free-storage"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = 1
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Minimum"
  threshold           = 2147483648 # 2GiB
  dimensions          = { DBInstanceIdentifier = "${var.project_name}-mysql" }
  treat_missing_data  = "notBreaching"
  alarm_description   = "RDSの空き容量が2GiBを下回った"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  tags                = local.tags
}

# db.t4g.micro は小さく、CPUが張り付くと一気に詰まる。
resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  alarm_name          = "${var.project_name}-rds-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = 300
  statistic           = "Average"
  threshold           = 80
  dimensions          = { DBInstanceIdentifier = "${var.project_name}-mysql" }
  treat_missing_data  = "notBreaching"
  alarm_description   = "RDSのCPUが80%を10分超えている"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  tags                = local.tags
}
