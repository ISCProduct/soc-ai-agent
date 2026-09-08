# Discord連携: 本番の起動/停止セットアップ手順

最終更新: 2026-09-08

Discordのスラッシュコマンドから本番環境(AWS ECS Fargate + RDS)を操作する機能のセットアップ手順。

| コマンド | できること | 権限 |
|---|---|---|
| `/prod state:on` | **今すぐ起動**し、以後も起動し続ける | `DISCORD_ALLOWED_ROLE_ID` のロール保有者のみ |
| `/prod state:off` | **今すぐ停止**し、以後も停止し続ける | 同上 |
| `/prod state:auto` | 日付リストに従う状態へ戻す(既定) | 同上 |
| `/prod-uptime` | 終日起動する日付を追加(モーダル入力) | 同上 |
| `/prod-uptime-list` | 起動予定日と現在の設定を表示 | 制限なし(誰でも閲覧可) |

## 仕組み

状態は SSM Parameter Store の2つのパラメータで決まる。

| パラメータ | 値 | 意味 |
|---|---|---|
| `/soc-app/prod-uptime-override` | `on` / `off` / `auto` | 手動オーバーライド。**日付リストより優先** |
| `/soc-app/prod-uptime-dates` | `2026-09-01,2026-09-02` | 終日起動する日付(JST、カンマ区切り) |

```
Discordで /prod state:on
        ↓ (Discord Interactions Webhook, HTTPS POST)
staging backend: POST /api/discord/interactions
  - Ed25519署名検証(DISCORD_PUBLIC_KEY)
  - 実行者のロールIDを確認(DISCORD_ALLOWED_ROLE_ID)
  - SSM /soc-app/prod-uptime-override に on を書く
  - GitHub Actions を workflow_dispatch で即時起動(GITHUB_DISPATCH_TOKEN)
        ↓
GitHub Actions: prod-uptime-scheduler.yml (毎時cron + 即時起動)
  1. override を読む。on なら起動、off なら停止(日付リストは見ない)
  2. auto なら日付リストと「今日(JST)」を照合
  3. 起動: RDS起動 → available待ち → chroma → rag-review → backend → frontend
     停止: ECSを0にしてから RDS停止
```

### なぜ ECS を直接叩かないのか

`prod-uptime-scheduler.yml` が毎時 desired_count を上書きするため、Discordから直接
ECSを起動しても**最大1時間で元に戻される**。「Discordで起動したのに落ちている」状態に
なるので、オーバーライドをSSMに書いてスケジューラに従わせる。

反映処理そのものをGoに書き直さないのも同じ理由で、RDSの起動待ちやサービスの起動順を
二重管理すると片方だけ直して本番が中途半端に起動する事故につながる。

### GITHUB_DISPATCH_TOKEN が無い場合

`/prod` は動作し、SSMのオーバーライドは書き込まれる。ただし**反映は次の毎時実行
(最大1時間後)**になる。Discordの応答にもその旨が表示される。

なぜstaging backendか: 本番(prod)は既定停止のため常時起動しているサーバーが必要。stagingは
常時稼働方針のため、ここにDiscord Interactions Endpointを追加する。

## 0. 反映の順序（重要）

`prod-uptime-scheduler.yml` は **cron / workflow_dispatch ともデフォルトブランチ(main)の
定義で動く**。オーバーライドを読む変更が main に入る前に `/prod` を使えるようにすると、
コマンドは成功したように見えて SSM に書かれるだけで**本番の状態は何も変わらない**。
「off にしたのに本番が起動したまま課金される」状態になる。

したがって次の順で進めること。

1. **CIのIAMユーザーに override パラメータの読み取りを許可する**（手順6。これが先。
   足りないと毎時のジョブが失敗し続ける。本番アカウントでは適用済み）
2. スケジューラの変更を **main まで反映**する（develop → release → main）
3. staging に backend を反映する（`terraform apply` または CI デプロイ）
4. `register-commands.sh` を実行して `/prod` を登録する

`/prod` を登録するのは最後。登録しなければ誰も実行できないので、これが安全弁になる。

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

# 任意: /prod の即時反映用。未設定なら次の毎時実行まで待つ
github_dispatch_token = "<GitHub Fine-grained PAT>"
github_dispatch_repo  = "ISCProduct/soc-ai-agent"
```

`github_dispatch_token` は **Fine-grained PAT** を推奨する。必要な権限は対象リポジトリの
`Actions: Read and write` のみ。これだけあれば `workflow_dispatch` を呼べる。
リポジトリのコード読み書き権限は不要なので付けないこと。

`terraform apply` で staging EC2 の `.env` に反映される（`docker compose up -d app` 相当の
再起動で反映、または次回CIデプロイで自動反映）。

## 5. スラッシュコマンドの登録

一度だけ実行（コマンド内容を変更した場合のみ再実行）:

```bash
DISCORD_BOT_TOKEN=<Botトークン> DISCORD_APPLICATION_ID=<Application ID> \
  ./automation/discord/register-commands.sh
```

## 6. SSM Parameter Store 読み書き権限

### ⚠️ GitHub Actions 側のIAMに override のARNを足す（必須）

CIが使うIAMユーザーのインラインポリシー `AllowProdUptimeSsmRead` は
`prod-uptime-dates` **だけ**にリソース限定されており、そのままでは
`prod-uptime-override` が `AccessDeniedException` になる（2026-09-08 実測）。

この状態でも日付リストどおりの起動は続くが（縮退動作）、`/prod` は効かず、
毎時のジョブは失敗し続ける。**main へ反映する前に**次を実行すること。

> **本番アカウント(508897596159)では 2026-09-09 に適用済み。**
> 動作確認まで完了しているので、既存環境では再実行不要。
> 別アカウント・別IAMユーザーで動かす場合のみ必要。

```bash
aws iam put-user-policy --user-name <CIのIAMユーザー> \
  --policy-name AllowProdUptimeSsmRead \
  --policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Sid": "ProdUptimeSsmRead",
      "Effect": "Allow",
      "Action": "ssm:GetParameter",
      "Resource": [
        "arn:aws:ssm:ap-northeast-1:<アカウントID>:parameter/soc-app/prod-uptime-dates",
        "arn:aws:ssm:ap-northeast-1:<アカウントID>:parameter/soc-app/prod-uptime-override"
      ]
    }]
  }'
```

確認:

```bash
aws ssm get-parameter --name /soc-app/prod-uptime-override
# ParameterNotFound なら権限はOK（パラメータは初回 /prod 実行時に作られる）
# AccessDeniedException ならポリシーが効いていない
```

### staging backend 側


staging EC2のIAMロールには `infra/terraform/environments/staging/main.tf` の
`ProdUptimeSsmAccess` ステートメントで `/soc-app/prod-uptime-dates` と
`/soc-app/prod-uptime-override` への `ssm:GetParameter` / `ssm:PutParameter` が
付与済み（staging tfvars変更後は `terraform apply` が必要）。

GitHub Actions側（`prod-uptime-scheduler.yml`）は既存の `AWS_ACCESS_KEY_ID` /
`AWS_SECRET_ACCESS_KEY` シークレットを使用する。このIAMユーザーに
`ssm:GetParameter`、`ecs:DescribeServices` / `ecs:UpdateService`、
`rds:DescribeDBInstances` / `rds:StartDBInstance` / `rds:StopDBInstance`、
`application-autoscaling:DescribeScalableTargets` / `RegisterScalableTarget`
の権限が必要。

## 動作確認

### /prod（起動・停止）

1. Discordで `/prod state:off` を実行 → 「🛑 常時停止に設定しました」を確認
2. `aws ssm get-parameter --name /soc-app/prod-uptime-override` が `off` になっていることを確認
3. GitHub Actions の `Prod uptime scheduler` が起動し、ECSの desired_count が 0、
   RDSが停止することを確認
4. `/prod state:on` で逆方向を確認（RDSの起動待ちがあるため数分かかる）
5. 展示会などが終わったら `/prod state:auto` で日付リスト運用へ戻す

### /prod-uptime（日付登録）

1. Discordで `/prod-uptime` を実行 → モーダルが表示されることを確認
2. 日付（例: 明日の日付）を入力して送信 → 「✅ 追加しました」のメッセージを確認
3. `aws ssm get-parameter --name /soc-app/prod-uptime-dates` で登録内容を確認
4. `prod-uptime-scheduler.yml` を `workflow_dispatch` で手動実行し、ログでECSサービスの
   `desired_count` が更新されることを確認

## 既知の制約

- **`GITHUB_DISPATCH_TOKEN` は本番デプロイも起動できてしまう。** `deployment.yml` にも
  `workflow_dispatch` があり、fine-grained PAT を「このワークフローだけ」に絞る手段が無い。
  トークンは staging EC2 の `.env` と Launch Template の user_data に**平文**で載るため、
  staging が侵害されると本番デプロイを起動されうる。許容できない場合はトークンを設定せず、
  反映を毎時実行に任せること（`/prod` は設定なしでも動作する）

- **`/prod state:on` のまま放置すると本番が課金され続ける。** 用が済んだら `auto` に
  戻すこと。`/prod-uptime-list` に「常時起動に固定されています」と表示されるので、
  定期的に確認する
- 起動には**数分かかる**（RDSの起動待ちを含む）。Discordの応答は「反映を開始しました」
  までで、完了通知は無い。実際の状態は GitHub Actions のログで確認する
- `/prod state:off` は確認ダイアログ無しで即座に本番を止める。ロール制限が唯一の防御
- デプロイ作業中との競合を防ぐメンテナンスロックは未実装（`prod-uptime-scheduler.yml` 内に
  `ponytail:` コメントで明記）。実運用で問題が出た場合は追加検討する
- 日付はJSTの暦日（00:00〜23:59:59）単位。時刻指定はできない
- 過去日は登録できない

## テスト

起動/停止の判定は `automation/test/prod-uptime-decision-test.sh` が
`prod-uptime-scheduler.yml` から判定部分を抜き出して実行する（CIの `Workflow Scripts`
ジョブで実行）。判定を間違えると展示会当日に本番が落ちたまま、または止めたはずの本番が
課金され続けるため、ワークフローを直したらこのテストも確認すること。
