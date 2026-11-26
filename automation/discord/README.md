# Discord 進捗リマインダー

指定チャンネルへ「進捗を入力してください」と通知するだけのツール。
メンバーは通常メッセージで進捗を投稿する。メッセージの読み取り・収集・保存は行わない。
Bot登録やBot Tokenは不要で、Python 3の標準ライブラリとIncoming Webhookだけを使う。

## 設定

`DISCORD_PROGRESS_WEBHOOK_URL` を環境変数で設定するか、
このディレクトリの `.env.progress.local` に設定する。
提供されたWebhookは同ファイルに保存済み（Git管理外・パーミッション0600）。
環境変数が優先される。URLはログやGitに記録しない。

通知先はWebhookが紐づくチャンネル。通知文の冒頭に `@everyone` を付け、
`allowed_mentions.parse` に `everyone` を設定してメンションを有効化する。
個別ユーザー・ロールのメンション解析は有効にしない。

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

通知には「今週の進捗」「次にやること」「困っていること・相談」のMarkdownテンプレートを
コードブロックで載せる。コピーして記入し、通常メッセージとして投稿できる。
テンプレート内には `@everyone` を含めず、報告のたびに全員へ通知することを防ぐ。

`wait=true` で投稿確認ができた場合のみ成功終了する。
失敗時は終了コード1。タイムアウト等で送信済みの可能性があるため、自動再送しない。
チャンネルの履歴を確認してから再実行する。HTTP 429の場合は時間を置く。
## 毎週日曜日18:07（日本時間）の定期通知

GitHub Actions の `Discord Progress Reminder` が毎週日曜日09:07 UTC
（18:07 JST）に実行する。毎時0分はスケジュールが欠落しやすいため7分にずらしている
（2026-09-27 に `0 9 * * 0` が発火しなかった）。Macの稼働状態には依存しない。
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

## Discord ↔ Backlog の紐づけ（/link）

Backlog の期限通知でメンションを飛ばす・`/deadline` で本人確認をするための対応表。
管理者ロール（`DISCORD_ALLOWED_ROLE_ID`）だけが操作できる。

| コマンド | 内容 |
|---|---|
| `/link member:@誰か backlog:名前 github:ログイン名` | 紐づける。`github` は任意。朝のリマインドで、その人にレビューが来ている PR をメンションする |
| `/unlink member:@誰か` | 解除する |
| `/link-list` | 一覧を本人にだけ表示する。2000文字を超えるときは `page` で続きを見る |

- 表は DynamoDB（`${project}-discord-backlog-links`、1人1項目）に持つ。突き合わせは Backlog のユーザーID。
- 以前の SSM Parameter `/soc-app/discord-backlog-links` は、初回の読み出しで DynamoDB へ移す。移したあとは SSM へ書き戻さない。
- 同じ Backlog ユーザー、同じ GitHub ログイン名（大文字小文字は区別しない）を別の Discord ユーザーに紐づけることはできない（先に `/unlink`）。
- 人数の上限は置かない。別の人の `/link` が同時でも、相手の行は消えない。
- 有効にするには Secrets Manager `<project>/discord-backlog-bot` に Bot ユーザーの API キーが要る。未設定なら `/link` 系は「利用できません」を返す。個人のキーは Lambda に置かない。
- コマンドの追加後に `automation/discord/register-commands.sh` を再実行する。

## Backlog の期限リマインド

平日 9:30 JST に、期限が2日以内・期限切れの課題を担当者ごとにまとめて投稿する（EventBridge Scheduler → discord Lambda）。

- 対象ステータス: 未対応・処理中・PR確認待ち。保留・処理済み・ステージング反映以降は対象外。
- 担当者は `/link` で紐づけた人にメンションする。未登録の人は名前だけで、「Discord未登録」と出る。
- 「PR確認待ち」の課題があるときは、レビュアー（terraform の `backlog_reviewer_ids`、既定は大橋和幸）にもメンションして、レビューを促す。レビュアー本人が担当の課題は対象にしない。
- 開いている PR のうち、レビュー待ちのものも同じ投稿に載せる。Draft と、最新のコミットに変更要求が付いたまま更新が無い PR は除く。レビューは履歴なので、人ごとに最新の意見（承認・変更要求・却下）で判断する（変更要求のあとに承認した人は載せない）。
- 個人宛てのレビュー依頼だけを拾う。チーム宛ての依頼は載らない。開いている PR は更新が新しい順に50件まで（超えたら CloudWatch Logs に警告を出す）。0件ならその節は出さない。GitHub の取得に失敗しても、Backlog の期限リマインドは投稿する。
- メンション先は `/link` の `github`（GitHub のログイン名）で引く。未設定の人はログイン名だけで、「Discord未登録」と出る。
- 取得用のトークンは `GITHUB_PR_READ_TOKEN`（Pull requests の読み取りだけ）。`/prod` の配信用 `GITHUB_DISPATCH_TOKEN` は使わない。同じ値を入れると節を出さない。対象リポジトリは `GITHUB_DISPATCH_REPO`（terraform の `github_dispatch_repo`）と同じ。トークンが無ければ節を出さない。
- 課題もレビュー待ち PR も無い日は何も投稿しない。
- 投稿先は Webhook（terraform の `discord_reminder_webhook_url`）。未設定なら動かない。
- 手動確認（投稿しない）: Lambda を `{"source":"backlog-deadline-reminder","dry_run":true}` で invoke すると、投稿予定の本文が返る。
- 再試行はしない（成功後の失敗で二重投稿になるため）。失敗は CloudWatch Logs `/aws/lambda/<project>-discord` の `[Reminder]` に残る。

## 期限の変更（/deadline）

間に合わないときに、担当者本人が Discord から期限を変更する。Backlog の期限日を更新し、変更履歴をコメントに残す。

```
/deadline issue:SOCAIAGENT-403 date:11/12 reason:レビュー待ちのため
```

- `issue` は、入力すると自分が担当の課題が候補に出る。`date` は `2026-11-12` または `11/12`。
- **本人確認**: Discord のユーザーIDを `/link` の対応表で Backlog ユーザーに引き当て、その人が課題の担当者のときだけ反映する。他人の課題・担当者なしの課題は変更できない。
- 変更できるステータスは、未対応・処理中・PR確認待ち・保留。処理済み以降は変更できない。
- 過去の日付と、今日から180日より先の日付は受け付けない。年を省いた日付が過ぎていたときも拒否する（来年は `2027-01-05` のように年から入力）。
- 結果はチャンネルに公開で返る（理由の `@` は無効化し、メンションの通知は飛ばさない）。失敗は本人にだけ表示する。
- Backlog への更新は、Bot ユーザーの名前で記録される。変更者はコメント本文に残る。Bot に課題の編集権限が無いときは「更新権限がありません」と返す。
- コマンドの追加後に `register-commands.sh` を再実行する。

## Bot 用 API キー（SOCAIAGENT-510）

Discord から Backlog を読む・更新するキーは、個人の API キーではなく Bot ユーザーのキーにする。置き場所は Secrets Manager `<project>/discord-backlog-bot`（staging 既定は `soc-stg/discord-backlog-bot`）。中身は JSON ではなくキー文字列そのもの。Lambda にはシークレット名だけを渡し、値は起動時に読む。

初回の `terraform apply` で `backlog_bot_api_key_plain` を渡すと、その値が入る。空なら `replace-via-cli` が入り、その間は `/link` と `/deadline` は動かない。2回目以降の apply は値を変えない。

ローテーションは AWS CLI で行う。`terraform apply` では書き換わらない。

```bash
umask 077
# NEW_SECRET はシェルにだけ置く。コマンドの引数には出さない。
printf '%s' "$NEW_SECRET" > /tmp/backlog-bot-key
aws secretsmanager put-secret-value \
  --secret-id soc-stg/discord-backlog-bot \
  --secret-string file:///tmp/backlog-bot-key
rm -f /tmp/backlog-bot-key
```

Lambda は起動時に読んだキーを持ち続ける。書き換えたあとは実行環境を入れ替える（次のデプロイか、`aws lambda update-function-configuration` で設定を一度更新する）。

Bot ユーザーと API キーの発行は Backlog の画面で行う。ここにはキーを書かない。

## 実機確認（SOCAIAGENT-512）

受け口は staging の Lambda です。書き込みの確認は、変更してよい課題を1つ決めてから行います。終わったら期限を元に戻します。

1. `/deadline` でその課題の期限を動かす。Backlog の期限が変わり、コメントが残る。
2. 同じコマンドで元の期限に戻す。
3. 拒否を確認する。未紐づけのメンバー、他人の課題、過去の日付、今日から180日より先。いずれも本人にだけ失敗が返る。
4. 朝のリマインドは、先に `{"source":"backlog-deadline-reminder","dry_run":true}` で本文を見る。投稿してよいと分かってから、スケジュールを有効にする。

## 止める・障害に気付く

朝のリマインドを止めるときは、スケジュールを `DISABLED` にする。staging は `soc-stg-backlog-deadline-reminder`、本番は `discord_reminder_schedule_enabled = false` で apply する。再試行はしないので、止めたあとに同じ通知が追い打ちされることはない。

失敗は CloudWatch Logs `/aws/lambda/<project>-discord` の `[Reminder]` と `[Discord]` に残る。本番で受け口を有効にしているときは、Lambda の Errors が1件でも SNS `<project>-alarms`（`alarm_email`）へ届く。`alarm_email` が空だと、アラームは状態を持つだけでメールは飛ばない。

本番の受け口は `discord_bot_enabled = true` で作る。既定は作らない。両方のスケジュールを ENABLED のままにすると、平日の朝に同じ内容が2回投稿される。

### 本番への切り替え手順

順番を守る。**apply → 実コードの配置 → Interactions Endpoint の切り替え**。

1. **事前準備**
   - staging の朝リマインド（`soc-stg-backlog-deadline-reminder`）を先に止める。
   - `BACKLOG_API_KEY` の渡し方を揃える。いまの prod は terraform 変数 `backlog_api_key` を Lambda の環境変数に直接入れている。有効にする前に、#2563 の `DISCORD_BACKLOG_BOT_SECRET_ID`（Secrets Manager から読む方式）に合わせる。この PR では実装しない。
2. **apply**: `infra/terraform/environments/prod` で `discord_bot_enabled = true` にして apply する。
   - 初回は `archive` provider が増えているため、先に `terraform init -upgrade` が要る（`infra/terraform/environments/prod/README.md` も参照）。
   - この時点の Lambda は **プレースホルダの bootstrap zip**（実体のない zip）。このまま Endpoint を切り替えると応答できず、Discord の検証に失敗する。
3. **実コードの配置**: 次のどちらかで `cmd/discord-lambda` のビルド成果物を入れる。
   - 手動（CLI）:
     ```bash
     cd Backend
     GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bootstrap ./cmd/discord-lambda
     zip -j discord-lambda.zip bootstrap
     aws lambda update-function-code --function-name <project>-discord --zip-file fileb://discord-lambda.zip
     aws lambda wait function-updated --function-name <project>-discord
     ```
   - CI: `deployment.yml` の `deploy-discord-lambda-prod` だけを再実行する（`main` の Deploy 実行から該当ジョブを Re-run）。
     Deploy workflow を `workflow_dispatch` で実行すると `deploy-production`（ECS 本番デプロイ）も一緒に走る点に注意する。
   - apply より前に `main` へ入った場合、関数が無いので `deploy-discord-lambda-prod` は ResourceNotFoundException のときだけスキップする。それ以外の AWS エラーはジョブが失敗する。
4. **Interactions Endpoint の切り替え**: 実コードが入ったことを確認してから、Discord Developer Portal の Interactions Endpoint URL を本番の URL に変える。`register-commands.sh` が必要なら併せて再実行する。
5. 朝リマインドを有効にするのは、切り替えを確認してから（`discord_reminder_schedule_enabled = true`）。

