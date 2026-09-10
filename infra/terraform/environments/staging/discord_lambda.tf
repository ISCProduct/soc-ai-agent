# Discord Interactions Endpoint の受け口（Lambda Function URL）。
#
# 受け口を staging の backend からここへ移した。staging はデプロイ後1時間で
# 自動停止するため、本番を起動する /prod がその間使えず「アプリケーションは
# 時間内に応答しませんでした」になっていた。本番の起動/停止を指示する口が、
# 止まる環境に乗っていてはいけない。
#
# コストについて:
#   - Lambda は月100万リクエスト + 40万GB-秒の永久無料枠がある。
#     想定利用（Discordコマンド 月数百回）では完全に無料枠内。
#   - Function URL を使うので API Gateway は不要（追加料金なし）。
#   - SSM も GitHub API もパブリックエンドポイントなので VPC には入れない。
#     VPC に入れると NAT Gateway が必要になり月$40級の固定費が発生する。
#
# staging 環境のディレクトリに置いているのは、必要な変数（discord_public_key 等）が
# ここに揃っているため。リソース自体は staging の起動状態とは独立して動く。

resource "aws_cloudwatch_log_group" "discord_lambda" {
  name              = "/aws/lambda/${var.project_name}-discord"
  retention_in_days = 14

  tags = local.tags
}

resource "aws_iam_role" "discord_lambda" {
  name = "${var.project_name}-discord-lambda-role"

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
  name = "${var.project_name}-discord-lambda-policy"
  role = aws_iam_role.discord_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = ["${aws_cloudwatch_log_group.discord_lambda.arn}:*"]
      },
      {
        # /prod の日付リストと手動オーバーライド、/staging の起動状態。
        # 対象パラメータは本番プロジェクト名(soc-app)固定。
        Sid    = "UptimeParameters"
        Effect = "Allow"
        Action = ["ssm:GetParameter", "ssm:PutParameter"]
        Resource = [
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/prod-uptime-dates",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/prod-uptime-override",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/soc-app/staging-uptime",
        ]
      },
    ]
  })
}

# 実体のバイナリは CI（deployment.yml）が update-function-code で入れる。
# terraform 実行環境に Go のクロスコンパイルを持ち込まないため、ここでは
# 関数の「箱」だけ作り、中身はプレースホルダにしておく。
data "archive_file" "discord_lambda_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/discord-lambda-placeholder.zip"

  source {
    content  = "placeholder: CI が update-function-code で実体を入れる"
    filename = "bootstrap"
  }
}

resource "aws_lambda_function" "discord" {
  function_name = "${var.project_name}-discord"
  role          = aws_iam_role.discord_lambda.arn

  # provided.al2023 + arm64。Go はカスタムランタイムで動かす。
  # arm64 の方が x86 より単価が安い（無料枠内なので実質差は出ないが、揃える理由もない）。
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"

  filename         = data.archive_file.discord_lambda_placeholder.output_path
  source_code_hash = data.archive_file.discord_lambda_placeholder.output_base64sha256

  # Discord は3秒で諦めるが、Lambda 側で先に切ると原因がログに残らない。
  # アプリ側で 2.5 秒の打ち切りを持っているので、ここは余裕を取る。
  timeout     = 10
  memory_size = 128

  environment {
    variables = {
      DISCORD_PUBLIC_KEY      = var.discord_public_key
      DISCORD_ALLOWED_ROLE_ID = var.discord_allowed_role_id
      GITHUB_DISPATCH_TOKEN   = var.github_dispatch_token
      GITHUB_DISPATCH_REPO    = var.github_dispatch_repo
    }
  }

  logging_config {
    log_format = "Text"
    log_group  = aws_cloudwatch_log_group.discord_lambda.name
  }

  # CI が入れたバイナリを terraform apply で巻き戻さない。
  lifecycle {
    ignore_changes = [filename, source_code_hash]
  }

  depends_on = [aws_iam_role_policy.discord_lambda]

  tags = local.tags
}

# 認証なしで公開する。Discord からの正当性は Ed25519 署名検証で確かめており、
# AWS の認証を挟むと Discord 側が署名を付けられず疎通できない。
resource "aws_lambda_function_url" "discord" {
  function_name      = aws_lambda_function.discord.function_name
  authorization_type = "NONE"
}

output "discord_interactions_endpoint" {
  description = "Discord Developer Portal の INTERACTIONS ENDPOINT URL に設定する値"
  value       = aws_lambda_function_url.discord.function_url
}
