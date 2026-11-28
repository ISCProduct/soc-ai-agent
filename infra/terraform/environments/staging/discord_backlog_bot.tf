# SOCAIAGENT-510: Discord から Backlog を更新する API キーは Bot ユーザー専用。
# 個人のキーは Lambda の環境変数に置かない。
# 値は Secrets Manager が正。初回 apply の backlog_bot_api_key_plain だけを書き、
# 以降は ignore_changes のため AWS CLI でローテーションする。
# 手順は automation/discord/README.md。

resource "aws_secretsmanager_secret" "discord_backlog_bot" {
  name        = "${var.project_name}/discord-backlog-bot"
  description = "Discord Bot が Backlog を更新する Bot 専用 API キー。個人のキーは置かない (SOCAIAGENT-510)"
  tags        = local.tags
}

resource "aws_secretsmanager_secret_version" "discord_backlog_bot" {
  secret_id = aws_secretsmanager_secret.discord_backlog_bot.id
  # 空のまま作ると ignore_changes で空が固定される。未設定の印は Lambda と同じ文字列。
  secret_string = var.backlog_bot_api_key_plain != "" ? var.backlog_bot_api_key_plain : "replace-via-cli"

  lifecycle {
    ignore_changes = [secret_string]
  }
}
