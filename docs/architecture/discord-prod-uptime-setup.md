# Discord連携: 本番「指定日終日起動」セットアップ手順

最終更新: 2026-08-17

`infra-decision-oci-stg-aws-prod.md` の本番起動ポリシー「(B) 指定日は終日起動」を、
Discordのスラッシュコマンド `/prod-uptime` から操作できるようにする機能のセットアップ手順。

## 仕組み

```
Discordで /prod-uptime → モーダルで日付(YYYY-MM-DD)入力 → 送信
        ↓ (Discord Interactions Webhook, HTTPS POST)
staging backend: POST /api/discord/interactions
  - Ed25519署名検証(DISCORD_PUBLIC_KEY)
  - 実行者のロールIDを確認(DISCORD_ALLOWED_ROLE_ID)
  - AWS SSM Parameter Store (/soc-app/prod-uptime-dates) に日付を追加
        ↓
GitHub Actions: prod-uptime-scheduler.yml (毎時cron)
  - SSM Parameterの日付リストと「今日(JST)」を照合
  - 該当すれば本番ECS Fargate(backend/frontend)の desired_count を 1 に、
    該当しなければ 0 に更新
```

なぜstaging backendか: 本番(prod)は既定停止のため常時起動しているサーバーが必要。stagingは
常時稼働方針のため、ここにDiscord Interactions Endpointを追加する。

## 1. Discord Application / Bot の作成

1. [Discord Developer Portal](https://discord.com/developers/applications) で **New Application** を作成
2. **General Information** タブで以下を控える:
   - `APPLICATION ID`
   - `PUBLIC KEY`（terraform変数 `discord_public_key` に設定する）
3. **Bot** タブで **Add Bot** → Token を発行し控える（`register-commands.sh` 実行時のみ使用。恒久保存は不要）
4. **Installation** タブ（または OAuth2 URL Generator）で以下を選択し、生成されたURLでBotをサーバーに招待:
   - Scopes: `applications.commands`
   - （ボタン/コマンド実行のみなら `bot` スコープや追加権限は不要）

## 2. Interactions Endpoint URL の設定

**General Information** タブの `INTERACTIONS ENDPOINT URL` に以下を設定:

```
https://api-stg.shukatsu-ai.jp/api/discord/interactions
```

保存時にDiscordがPING検証リクエストを送るため、事前に3〜6の設定を完了させ、staging backendが
起動している状態で保存すること。検証に失敗する場合は `DISCORD_PUBLIC_KEY` の設定漏れを疑う。

## 3. 実行権限ロールの確認

コマンドを実行してよいDiscordロールのIDを控える（Discordのサーバー設定 > ロール > 対象ロールを
右クリック > IDをコピー。開発者モードの有効化が必要）。

## 4. terraform.tfvars への設定（staging）

`infra/terraform/environments/staging/terraform.tfvars` に追記:

```hcl
discord_public_key      = "<Discord Developer PortalのPUBLIC KEY>"
discord_allowed_role_id = "<実行を許可するロールID>"
```

`terraform apply` で staging EC2 の `.env` に反映される（`docker compose up -d app` 相当の
再起動で反映、または次回CIデプロイで自動反映）。

## 5. スラッシュコマンドの登録

一度だけ実行（コマンド内容を変更した場合のみ再実行）:

```bash
DISCORD_BOT_TOKEN=<Botトークン> DISCORD_APPLICATION_ID=<Application ID> \
  ./automation/discord/register-commands.sh
```

## 6. SSM Parameter Store 読み書き権限

staging EC2のIAMロールには `infra/terraform/environments/staging/main.tf` の
`ProdUptimeSsmAccess` ステートメントで `/soc-app/prod-uptime-dates` への
`ssm:GetParameter` / `ssm:PutParameter` が付与済み（staging tfvars変更後は
`terraform apply` が必要）。

GitHub Actions側（`prod-uptime-scheduler.yml`）は既存の `AWS_ACCESS_KEY_ID` /
`AWS_SECRET_ACCESS_KEY` シークレットを使用する。このIAMユーザーに
`ssm:GetParameter` と `ecs:DescribeServices` / `ecs:UpdateService` の権限が
必要（既存デプロイ用ポリシーで大半はカバーされている想定。SSM権限が無ければ追加すること）。

## 動作確認

1. Discordで `/prod-uptime` を実行 → モーダルが表示されることを確認
2. 日付（例: 明日の日付）を入力して送信 → 「✅ 追加しました」のメッセージを確認
3. `aws ssm get-parameter --name /soc-app/prod-uptime-dates` で登録内容を確認
4. `prod-uptime-scheduler.yml` を `workflow_dispatch` で手動実行し、ログでECSサービスの
   `desired_count` が更新されることを確認

## 既知の制約

- デプロイ作業中との競合を防ぐメンテナンスロックは未実装（`prod-uptime-scheduler.yml` 内に
  `ponytail:` コメントで明記）。実運用で問題が出た場合は追加検討する
- 日付はJSTの暦日（00:00〜23:59:59）単位。時刻指定はできない
- 過去日は登録できない
