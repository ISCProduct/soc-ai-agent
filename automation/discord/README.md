# Discord 進捗リマインダー

指定チャンネルへ「進捗を入力してください」と通知するだけのツール。
メンバーは通常メッセージで進捗を投稿する。メッセージの読み取り・収集・保存は行わない。
Bot登録やBot Tokenは不要で、Python 3の標準ライブラリとIncoming Webhookだけを使う。

## 設定

`DISCORD_PROGRESS_WEBHOOK_URL` を環境変数で設定するか、
このディレクトリの `.env.progress.local` に設定する。
提供されたWebhookは同ファイルに保存済み（Git管理外・パーミッション0600）。
環境変数が優先される。URLはログやGitに記録しない。

通知先はWebhookが紐づくチャンネル。`@everyone` 等のメンションは送らない。

## 実行

リポジトリのルートで実行:

```bash
# 文面を確認（送信なし・認証情報不要）
python3 automation/discord/notify_progress.py --dry-run

# 通知を1回送信
python3 automation/discord/notify_progress.py

# テスト（実際のDiscordには送信しない）
python3 -m unittest discover -s automation/discord -p 'notify_progress_test.py' -v
```

通知内容は「取り組んだこと・進捗」「次にやること」「困っていること・相談したいこと」の
記入例。形式は任意で、フォームやコマンドによる進捗入力は不要。

`wait=true` で投稿確認ができた場合のみ成功終了する。
失敗時は終了コード1。タイムアウト等で送信済みの可能性があるため、自動再送しない。
チャンネルの履歴を確認してから再実行する。HTTP 429の場合は時間を置く。
## 毎週日曜日18:00（日本時間）の定期通知

GitHub Actions の `Discord Progress Reminder` が毎週日曜日09:00 UTC
（18:00 JST）に実行する。Macの稼働状態には依存しない。
GitHub側の混雑で開始が遅れる場合があり、厳密な定刻実行は保証されない。

設定ファイル: `.github/workflows/discord-progress-reminder.yml`

- Repository Secret `DISCORD_PROGRESS_WEBHOOK_URL` にWebhookを登録する。
- ワークフローをデフォルトブランチ（main）へマージすると定期実行が有効になる。
- PRではテストとプレビューだけを実行し、通知用Secretは送信ステップだけに渡す。
- Actions → Discord Progress Reminder → Run workflow で手動実行できる。
  `dry_run` は既定でtrue（送信なし）。falseを選ぶと1回通知する。
- 停止する場合はActions画面から Disable workflow を選ぶ。
- 同じ実行を再実行すると通知も再送されるため、失敗時はDiscordの履歴を先に確認する。

Macで以前登録したlaunchdは、クラウドへの移行時に解除する。

```bash
launchctl bootout "gui/$(id -u)/jp.soc-ai-agent.progress-reminder"
rm ~/Library/LaunchAgents/jp.soc-ai-agent.progress-reminder.plist
```

仕様参照:
- https://docs.discord.com/developers/resources/webhook
- https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule
