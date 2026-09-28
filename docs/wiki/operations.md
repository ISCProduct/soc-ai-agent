# 運用手順書

## 目次

1. [デプロイ手順](#1-デプロイ手順)
   - [停止日の本番デプロイ](#11-停止日の本番デプロイ-1518)
2. [定期バッチ作業](#2-定期バッチ作業)
3. [監視項目](#3-監視項目)
   - [リクエストIDでサービス横断追跡](#31-リクエストidでサービス横断追跡-1188)
   - [メトリクスを見る](#32-メトリクスを見る-1186)
   - [稼働日の前日チェックリスト](#33-稼働日の前日チェックリスト-1388)
   - [エラートラッキング(Sentry)の有効化](#34-エラートラッキングsentryの有効化-1185)
   - [エラートラッキング（Sentry）](#34-エラートラッキングsentry619--1185)
   - [クライアントIPをBFFからBackendへ引き継ぐ](#35-クライアントipをbffからbackendへ引き継ぐ-1407)
4. [障害対応](#4-障害対応)
5. [データベース管理](#5-データベース管理)
6. [管理画面操作](#6-管理画面操作)

---

## 1. デプロイ手順

### Docker Compose（ステージング/本番）

```sh
# イメージビルド & 起動
docker compose up -d --build

# バックエンドのみ再起動
docker compose restart app

# ログ確認
docker compose logs -f app
```

### メール送信（Resend / #758）

本番・staging は独自ドメインを使う。`soc-ai-agent.com` ではなく検証済みの `shukatsu-ai.jp`。

```env
EMAIL_PROVIDER=resend
RESEND_API_KEY=re_xxxxxxxxx
EMAIL_FROM=noreply@shukatsu-ai.jp
```

Resend ダッシュボードで `shukatsu-ai.jp` が Verified であること。API キーは Secrets のみ（リポジトリに置かない）。試験送信は登録確認メールで確認する。

### ヘルスチェック確認

```sh
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

### マイグレーション

バージョン管理型マイグレーション（`Backend/migrations/`、`cmd/migrate`）を使用（GORMの `AutoMigrate` は禁止）。CI/CD（`.github/workflows/deployment.yml`）で以下の通り自動適用される（#618）。

- **staging**: `docker compose up -d` 後、backendコンテナ内で `docker compose exec -T backend /bin/migrate up` を実行。失敗するとデプロイジョブが失敗する。
- **production**: 新イメージのタスク定義を登録し、backendサービスと同じネットワーク設定でワンオフECSタスクとして `/bin/migrate up` を実行。マイグレーションが失敗した場合はサービスの更新（新イメージへの切り替え）自体を行わない。

手動実行する場合:

```sh
cd Backend && go run ./cmd/migrate            # up
cd Backend && go run ./cmd/migrate down       # 直近1件をロールバック
cd Backend && go run ./cmd/migrate version    # 現在のバージョン確認
```

### デプロイのロールバック（#618）

**backend/frontend（ECS Fargate, 本番）:**

直前の安定リビジョンに戻す。マイグレーションが絡む変更は、先に対応する `down` マイグレーションを実行してからサービスを戻す。

```sh
# 1. 直前のタスク定義リビジョンを確認
aws ecs list-task-definitions --family-prefix soc-app-backend --sort DESC --max-items 5

# 2. スキーマ変更を伴う場合、先にロールバック（本番DBに対して慎重に実行）
cd Backend && go run ./cmd/migrate down

# 3. サービスを直前のリビジョンへ戻す
aws ecs update-service --cluster soc-app --service backend \
  --task-definition soc-app-backend:<直前のリビジョン番号>
aws ecs update-service --cluster soc-app --service frontend \
  --task-definition soc-app-frontend:<直前のリビジョン番号>
```

**staging（EC2 + Docker Compose）:**

```sh
# IMAGE_TAG=staging は都度上書きされるため、直前に安定していたコミットへ再デプロイする
# のが確実（GitHub Actions を workflow_dispatch で該当コミットに対して再実行）
gh workflow run deployment.yml --ref <直前の安定コミットSHA>
```

**戻す前に、戻し先のイメージがECRに在ることを必ず確認する（#1518）。**
ECRのライフサイクルで失効していると、リビジョンは残っていても `CannotPullContainerError`
で起動できない。

```sh
img=$(aws ecs describe-task-definition --task-definition soc-app-backend:<リビジョン> \
  --query 'taskDefinition.containerDefinitions[0].image' --output text)
aws ecr describe-images --repository-name soc-backend \
  --image-ids imageTag="${img##*:}" --query 'imageDetails[0].imagePushedAt' --output text
```

---

## 1.1 停止日の本番デプロイ (#1518)

本番は「指定日のみ終日起動」なので、通常日のデプロイは **RDS とECSサービスを一時的に
起動して確認し、終わったら desired=0 とRDS停止へ戻す**。`deployment.yml` の
`deploy-production` ジョブが1ジョブの中で次の順に行う。

1. Playwright スモークの準備（`npm ci` / ブラウザ取得）
   ※**本番AWS認証情報を入れる前**に済ませる。`configure-aws-credentials` 以降は
   ECS/RDS/ECR を操作できるキーがジョブ全体の環境変数に残るため、その状態で
   `npm ci` を走らせると依存パッケージの install スクリプトから本番を触れる。
   スモーク本体のステップも `AWS_ACCESS_KEY_ID` などを空で上書きしてから実行する。
2. イメージのビルド / push（`--provenance=false`。理由は後述）
3. 新しいタスク定義を登録し、ワンオフタスクで `migrate up`（backend 変更時のみ）
4. 各サービスへ新しいタスク定義を適用
5. **一時起動**（`automation/ops/prod-scale.sh 1`）
6. ECSの安定待ち（一時起動したときは chroma を含む**全サービス**を待つ。
   変更のあったサービスだけ待つと、cold start 中の frontend をスモークが叩いて落ちる）
7. **Playwright スモーク**（`frontend/e2e/smoke/smoke.spec.ts`。結果は Discord へ）
8. **タスク定義のイメージ存在検査**（`automation/ops/verify-task-def-images.sh`、`always()`）
9. desired=0 へ戻す（`always()`、`override=on` と起動日は尊重する）
10. このデプロイが起動したRDSを停止（`always()`）。
    backend を変更していない停止日のデプロイでも 5 のために RDS を起動するので、
    停止の条件は「このデプロイが起動したか（`started_by_deploy`）」だけで見る。

**この5〜9の間（数分）、本番の公開URLが実際に生きる。** 停止日でも
`https://shukatsu-ai.jp` が応答するのはこのためで、スモークを足した分だけ
その時間は伸びた（従来の「起動確認だけ」より1〜2分程度）。

長時間ハングでRDSを起動したまま `concurrency` を占有しないよう、`npm ci` と
スモークのステップには `timeout-minutes: 15` を入れている。ジョブ全体ではなく
ステップに入れるのは、ジョブのタイムアウトだと後段の `always()`（desired=0 へ戻す /
RDS停止）まで打ち切られ、停止日の本番が起動したまま残るため。

### 毎時の起動/停止ジョブとの排他

`prod-uptime-scheduler.yml`（毎時5分）は、**本番デプロイが実行中なら停止処理を見送る**
（`gh run list --workflow deployment.yml --branch main` で確認）。起動処理は見送らない。

これが無かった 2026-09-25 は、デプロイの最中にスケジューラがRDSを停止し、起動した
backend がDBへ繋がらずヘルスチェックに落ち、ECSのサーキットブレーカーが旧リビジョンへ
ロールバックした。その旧イメージはECRから失効していたため、**backend が起動不能な
タスク定義を指したまま残った**（停止日なので即時の障害にはならず、次の稼働日に発覚する）。

見送りは「最大1時間、停止が遅れる」だけで、デプロイ自身が最後に必ず0へ戻す。

**判定できなかった場合（API障害・権限不足）は停止しない。** そのまま停止へ進むと、
上のロールバック事故をGitHub APIが不調なたびに再発させることになる。
代わりにスケジューラのジョブを失敗させ、Discord（`@here`）へ理由を流す。

確認と停止のあいだにデプロイが始まる競合を避けるため、確認は
**RDSを止める直前にもう一度**行う（ECSの縮退ループに1〜2分かかるため）。
GitHub Actions の `concurrency` を `deploy-production` と共有する形の厳密な排他は
採っていない。同一 group では「pending のジョブは後から pending になったジョブに
キャンセルされる」ため、毎時のこのジョブが main へのデプロイを取り消しうる。

### ロールバック先の健全性

ECSのデプロイサーキットブレーカーは「直前の安定デプロイ」へ戻すが、そのイメージが
ECRに在るかは見ない。そこで、デプロイの最後に
`automation/ops/verify-task-def-images.sh` が各サービスのタスク定義を検査し、
イメージが無ければ**今回のデプロイで登録したリビジョンへ戻したうえでジョブを失敗させる**。

ただし**自動で戻すのは desired=0 かつ running=0 のサービスだけ**。稼働日に
「タグはECRから失効しているが既存タスクは動き続けている」旧リビジョンへ
サーキットブレーカーが戻した場合、そこで新リビジョン（＝直前にヘルスチェックに
落ちたもの）へ向け直すと、`minimumHealthyPercent=0` のため健全な稼働タスクが先に
落とされて本番が停止する。稼働中は検査結果を出してジョブを失敗させるだけにし、
復旧は人が判断する。

手で確認・復旧する場合:

```sh
PROJECT_NAME=soc-app ./automation/ops/verify-task-def-images.sh \
  backend=arn:aws:ecs:ap-northeast-1:<account>:task-definition/soc-app-backend:<リビジョン>
```

### ECRのライフサイクル

ECRリポジトリ（`soc-backend` / `soc-frontend` / `soc-rag-review`）は **staging と本番で
共用**で、Terraform では `environments/staging` の `module "ecr"` が持つ。

| ルール | 対象 | 保持数 |
| --- | --- | --- |
| 1 | タグ無し（タグが外れた古い index / 古い buildcache 等） | `lifecycle_untagged_keep_count`（既定60） |
| 2 | タグ付き（リリースのSHAタグ / `staging` / `buildcache-*`） | `lifecycle_keep_count`（既定20） |

以前は `tagStatus=any` の1ルール（20件）だけだった。1ビルドで積まれるマニフェストは
4件（image index・その子のプラットフォーム manifest・attestation・buildcache）で、
**タグ無しの山がタグ付きのリリースイメージを枠から押し出していた**。
staging のデプロイ回数もそのまま本番イメージの寿命を削るため、実質4デプロイ程度で
前回の本番イメージが消えていた。

あわせてビルドに `--provenance=false` を付け、1タグあたりのマニフェストを
index+attestation の分だけ減らしている（provenance は誰も参照していない）。

---

## 2. 定期バッチ作業

### 週次推奨作業

| 作業 | APIエンドポイント | 頻度 | 説明 |
|------|-----------------|------|------|
| 企業プロファイル再計算 | `POST /api/admin/profile-recalculation/run` | 週1回 | 通過実績からCompanyWeightProfileを更新 |
| 集合知サマリー再集計 | `POST /api/admin/collective-insights/rebuild-summaries` | 週1回 | 企業別通過率サマリーを更新 |
| スコアキャリブレーション | `POST /api/admin/score-validation/calibration/run` | 月1回 | 通過率データからスコア重みを調整 |

### バッチ実行例（curl）

```sh
# 認証ヘッダーが必要（管理者メール/パスワードのBase64）
AUTH="Authorization: Basic $(echo -n 'admin@example.com:password' | base64)"

# 企業プロファイル再計算
curl -X POST http://localhost:8080/api/admin/profile-recalculation/run \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"min_samples": 3}'

# 集合知サマリー再集計
curl -X POST http://localhost:8080/api/admin/collective-insights/rebuild-summaries \
  -H "$AUTH"

# スコアキャリブレーション
curl -X POST http://localhost:8080/api/admin/score-validation/calibration/run \
  -H "$AUTH"
```

---

## 3. 監視項目

### APIコスト監視

OpenAI APIのコストを管理画面で確認できます:

```sh
GET /api/admin/costs/summary   # 総コストサマリー
GET /api/admin/costs/daily     # 日別コスト
GET /api/admin/costs/monthly   # 月別コスト
```

**アラート設定:**
- OpenAI API 全体（`api_call_logs`）: `OPENAI_COST_ALERT_THRESHOLD_USD=40`（UTC 月次）を超過すると Slack/Discord
  - `OPENAI_COST_ALERT_SLACK_WEBHOOK_URL` / `OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL`
  - Slack 未設定時は `REALTIME_ALERT_SLACK_WEBHOOK_URL` へフォールバック
  - 同一月は1回のみ通知
- Realtime: `REALTIME_MONTHLY_ALERT_THRESHOLD_USD=200` を超えるとメール/Slack
  - `REALTIME_ALERT_EMAILS` / `REALTIME_ALERT_SLACK_WEBHOOK_URL`

### 面接セッション監視

同時接続数の上限: `REALTIME_MAX_CONCURRENT_CONNECTIONS=30`

超過した場合はサーバーログに `[Realtime] connection limit reached` が出力されます。

### ログ確認コマンド

```sh
# バックエンドのエラーログ
docker compose logs app | grep -i "error\|failed\|panic"

# クロス機能連携の警告
docker compose logs app | grep "\[CrossFeature\]"

# 面接レポート生成の状況
docker compose logs app | grep "\[Interview\]"
```

---

## 3.1 リクエストIDでサービス横断追跡 (#1188)

1リクエストに1つのIDを振り、FE -> BE -> RAG のログを同じIDで突き合わせる。
追加のトレーシング基盤（OpenTelemetry / Jaeger）は導入していない。構造化ログの
突き合わせで足りるため、コスト削減方針と両立させている。

| 区間 | ヘッダー | 採番/検証 |
| --- | --- | --- |
| ブラウザ -> Next.js | `X-Request-ID` | `frontend/middleware.ts` が採番。クライアント指定値は `^[A-Za-z0-9_-]{1,64}$` のみ採用し、それ以外は採番し直す |
| Next.js -> Backend | `X-Request-ID` | Route Handler が `extractUserAuthHeaders` / `adminProxyHeaders` で転送 |
| Backend 内 | - | `middleware.RequestIDMiddleware` がコンテキストへ格納。レスポンスにも同ヘッダーを返す |
| Backend -> RAG | `X-Trace-ID` + `X-Request-ID` | `ragclient.SetAuthHeader` がコンテキストから付与 |
| RAG 内 | - | `_trace_id_middleware` が `trace_id` として構造化ログへ出力し、両ヘッダーで返す |

ユーザーから不具合報告を受けたときは、ブラウザのレスポンスヘッダー `X-Request-ID`
を聞き取り、各サービスのログを横断 grep する。

```sh
RID=<X-Request-ID の値>
docker compose logs app        | grep "$RID"
docker compose logs rag-review | grep "$RID"
```

バックグラウンド処理（求人情報のRAGへのpush等）はリクエストのキャンセルは
引き継がず、IDのみ引き継ぐ。呼び出し元リクエストが完了済みでも追跡できる。

### 現時点のカバレッジと制限

Next.js の Route Handler が Backend へ転送するのは、共通ヘルパー
（`extractUserAuthHeaders` / `adminProxyHeaders`）を使っている Handler だけ。
ヘッダーを自前で組んでいる Handler が **64本** 残っており、そこを通った
リクエストは Backend が別のIDを採番する。

つまりブラウザに返った `X-Request-ID` で grep しても、この64本経由の
リクエストは Backend のログに出てこない。その場合は時刻とパスで絞り込み、
Backend が採番したIDを拾ってから RAG のログを追う。

新しい Route Handler は共通ヘルパーを使うこと。
`frontend/tests/app/request-id-propagation.test.ts` が自前ヘッダーの
Handler 数を監視していて、増やすとテストが落ちる。
残りの64本の移行は #1188 のフォローアップとして別PRで行う。

---

## 3.2 メトリクスを見る (#1186)

Backend と RAG は Prometheus 形式のメトリクスを `/metrics` で出す。
**Prometheus / Grafana は常設していない。** 本番は稼働日のみ起動する運用なので、
監視スタックを常時動かすと稼働していない日も課金が続く。見たいときだけ手元で起動して
スクレイプする。

### 何が取れるか

| 指標 | 内容 |
|---|---|
| `backend_requests_total` | メソッド・パス・ステータス別のリクエスト数（RED の Rate / Errors） |
| `backend_request_duration_seconds` | レイテンシのヒストグラム（RED の Duration） |
| `http_request_duration_seconds`（RAG） | RAG 側の同等指標 |
| `go_*` / `process_*` | GC・メモリ・FD 数（USE の Utilization / Saturation） |

ALB のヘルスチェック（`/health`, `/healthz`）は計装から除外している。30秒ごとに来るため、
含めるとリクエスト数の大半を占めて実際のトラフィックが読めなくなる。

**ルートに一致しないリクエスト（404）も除外している。** この場合 Echo の `c.Path()` が空になり、
ライブラリは `url` ラベルへ生のパスを入れる。ALB はインターネット直結でスキャンを日常的に
受けるため、そのまま計装するとラベルの種類が無限に増え、プロセスとスクレイパのメモリを
食いつぶす。404 の総数を見たい場合は ALB 側の `HTTPCode_Target_4XX_Count` を使う。

一致したルートは `url="/api/users/:id"` のようにパターンで記録されるため、パスパラメータで
ラベルが増えることはない。

### 有効化

Backend は `METRICS_TOKEN` が設定されているときだけ `/metrics` を公開する（計装自体も行わない）。
backend の ALB はインターネットに直結しているため、既定では開けない。

```bash
# 値は Secrets Manager の soc-app/admin などに置き、タスク定義から注入する
METRICS_TOKEN=<ランダムな長い文字列>
```

RAG 側は追加設定不要。`/metrics` は既存の内部認証ミドルウェアの対象なので、
`X-Internal-Token`（`RAG_INTERNAL_TOKEN`）が必要になる。

### 取得する

```bash
# Backend
curl -H "Authorization: Bearer $METRICS_TOKEN" https://api.shukatsu-ai.jp/metrics

# RAG（VPC 内部からのみ。ALB 経由では公開していない）
curl -H "X-Internal-Token: $RAG_INTERNAL_TOKEN" http://<rag-host>:9000/metrics
```

### 手元の Prometheus でスクレイプする

```yaml
# prometheus.yml
scrape_configs:
  - job_name: soc-backend
    scheme: https
    static_configs:
      - targets: ["api.shukatsu-ai.jp"]
    authorization:
      type: Bearer
      credentials: "<METRICS_TOKEN>"
```

```bash
docker run --rm -p 9090:9090 \
  -v "$PWD/prometheus.yml:/etc/prometheus/prometheus.yml" \
  prom/prometheus
```

稼働日が増えて常時監視が必要になったら、ここを常設の Prometheus + Grafana に置き換える。
その判断はコストとの兼ね合いで決める。

常時計測している指標と信頼性目標は [SLO とアラート](./slo.md) を参照（計測ソースは ALB の
CloudWatch メトリクス）。

---

## 3.3 稼働日の前日チェックリスト (#1388)

本番は稼働日のみ起動する。**起動を毎時の GitHub cron に任せきりにしないこと。**

### なぜ前日に起動するのか

`prod-uptime-scheduler.yml` は `cron: "5 * * * *"` と書いてあるが、GitHub ホストの scheduled workflow は
ベストエフォートで、実測の発火間隔は **平均約4時間・最大5.5時間**だった（直近18間隔で1時間以内は0回、#1388）。

JST 0時に稼働日へ入っても、次の発火は数時間後になる。**当日の朝に本番が上がっていない可能性がある。**
#1355 の失敗通知は「ジョブが動いて失敗した」ときに鳴るもので、**発火しなければ鳴らない**。

EventBridge Scheduler へ移すまで（#1388）は、以下を手順として実施する。

### 前日（夕方〜夜）

1. **本番を起動する**

   Discord で `/prod state:on`

   これでオーバーライドが `on` になり、日付リストに関係なく起動する。

   **Discord の返信が「反映を開始しました」であることを確認する。**
   即時実行は `GITHUB_DISPATCH_TOKEN` / `GITHUB_DISPATCH_REPO` が揃っているときだけで、
   未設定・API 失敗時は「⚠️ 即時反映の起動に失敗しました。次の毎時実行(最大1時間後)で
   反映されます。」が返る。この手順は「最大1時間待ち」を避けるためのものなので、
   そこで気づけないと意味が無い。

2. **起動を確認する**（RDS の起動待ちがあるため10分ほど見る）

   ```bash
   curl -s -o /dev/null -w '%{http_code}\n' https://api.shukatsu-ai.jp/health   # 200 を期待
   curl -s -o /dev/null -w '%{http_code}\n' https://shukatsu-ai.jp               # 200 を期待
   aws ecs describe-services --cluster soc-app --services chroma rag-review backend frontend \
     --query 'services[].{n:serviceName,d:desiredCount,r:runningCount}' --output text
   ```

   **4サービス全部を見ること。** 起動ジョブは `chroma rag-review backend frontend` を
   回しており、ここから漏れると稼働日でも RAG 機能だけ落ちる（実際に起きている）。

3. **主要フローを1回ずつ手で通す**（チャット / 面接 / 履歴書レビュー）

   起動しただけでは、シークレットの参照ミスや外部APIキーの失効は分からない。
   実際に rev 70/71 は**存在しないシークレットARN**を参照しており、起動すら
   できない状態のまま停止中だったため誰も気づかなかった（#1371）。

### 当日

4. 朝いちばんで `/health` を確認する（上のコマンド）

   **`/health` は生存確認だけ**で、DB 接続も外部 API も見ていない
   （`cmd/server/main.go` のハンドラは無条件に 200 を返す）。
   RDS が落ちている・タスク定義が古い状態でも 200 になるので、
   これだけで「動いている」と判断しないこと。前日ステップ3を1本だけでも通すのが確実。
5. 異常があれば Discord の運用アラートチャンネルを確認する（#1355 の通知先）

### 終了後

6. **本番を停止する**

   Discord で `/prod state:off`（明示的に停止）

   **当日中に止めたいなら `auto` ではなく `off`。** 当日を日付リストに登録して
   いる場合、`auto` に戻しても JST 0時までは起動が続き、そこから先の停止も
   毎時 cron 頼みになる（この節が問題にしている遅延が停止側にそのまま効く）。
   `off` は即時実行されるので確実に落ちる。翌日以降も稼働日が続くなら `auto` でよい。

   **`on` のまま放置すると課金が続く。** 起動ジョブは `on` のとき毎回
   「常時起動に固定されています」と警告を出すので、実行ログにも残る。

### 停止中にやってはいけないこと

- **本番へのデプロイ**: デプロイはワンオフ ECS タスクで `migrate up` を実行するため、
  RDS が停止していると必ず失敗する。デプロイ前に RDS を起動しておくこと
  （ECS タスクまで起動する必要はない。RDS だけでよい）

  ```bash
  aws rds start-db-instance --db-instance-identifier soc-app-mysql
  ```

  `/prod state:on` でも起きるが、そちらは ECS も上げるので停止中のデプロイのためだけなら
  余計に課金される。

## 3.4 エラートラッキング（Sentry）（#619 / #1185）

本番の未処理エラーを Backend / Frontend / RAG から Sentry へ送る。
**DSN 未設定時はすべて no-op**（ローカル開発では依存しない）。

### 環境変数

| 変数 | 対象 | 説明 |
|---|---|---|
| `SENTRY_DSN` | Backend / RAG / FE(server) | プロジェクトの DSN |
| `NEXT_PUBLIC_SENTRY_DSN` | Frontend(browser) | ブラウザ用 DSN（公開してよい値）。**ビルド時に渡す必要がある**（下記） |
| `SENTRY_RELEASE` | 共通（任意） | リリース識別子（git SHA など）。リリース単位の追跡に使う |

`NEXT_PUBLIC_*` は実行時ではなく**ビルド時にバンドルへ埋め込まれる**。ブラウザ側を動かすには
GitHub のリポジトリシークレット `SENTRY_DSN_FRONTEND` を設定すること（deployment.yml が
`--build-arg` で Docker ビルドへ渡す）。未設定のままだと SDK は積まれるが初期化されず、
バンドルだけ増えて1件も送信されない。

DSN の形式にも注意。2023年以降に作られた組織の DSN は `https://oNNN.ingest.us.sentry.io/...`
のようにリージョンが入る。CSP（`frontend/next.config.ts`）は `https://*.sentry.io` を
許可しているのでどちらの形式でも通るが、ここを狭めるとブラウザからの送信が全部ブロックされる。
| `APP_ENV` | 共通 | `development` / `staging` / `production`（Sentry environment） |

DSN は Secrets Manager 等に置き、リポジトリには置かない。

### 送信しないもの

`beforeSend` で次を落とす（履歴書・チャット本文などの個人情報対策）:

- リクエストボディ / Cookie / QueryString
- `Authorization` / `X-Admin-Token` / `X-User-Token` / `X-Company-User-Token` / `X-Internal-Token`
- `X-Origin-Token`（CloudFront の経路証明。漏れると公開ALB直叩きで詐称XFFを署名させられる）/ `Referer`（遷移元のクエリにワンタイムトークンが載る）

一覧は frontend `frontend/lib/sentry.ts` の `SENSITIVE_HEADERS` と Backend
`Backend/internal/observability/sentry.go` の `sensitiveHeaders` の2箇所にある。
片方だけ直す事故を防ぐため、両者の食い違いは Go のテスト
（`TestSensitiveHeaders_フロントと同期`）で落ちる。

相関は既存の `X-Request-ID`（`request_id` タグ）で行う（3.1 節）。

### 通知

Sentry プロジェクトの Alert Rule で Discord / Slack へ転送する。
コストアラート（#604）や CloudWatch（`slo.md`）と通知先を揃える。

### 動作確認

1. staging に DSN を設定してデプロイ
2. 意図的に 500 を起こす（または Sentry の test event）
3. Sentry Issues にイベントが届き、`request_id` タグでログと突合できること

### 受け入れ条件との対応（#619）

| 受け入れ条件 | 状態 |
|---|---|
| 本番の未処理エラーが通知される | Sentry（本節）。DSN 設定と Alert Rule が必要 |
| 基本メトリクスがダッシュボードで確認できる | `/metrics`（3.2 節 / #1186）+ ALB CloudWatch |
| SLOとアラートルールが文書化されている | [slo.md](./slo.md)（#1187） |

---

## 3.4 エラートラッキング(Sentry)の有効化 (#1185)

**DSN を設定しない限り Sentry は何も送信しない。** コードは Backend / Frontend / RAG の3つに入っているが、
いずれも DSN 未設定なら初期化自体をスキップする（`frontend/lib/sentry.ts` の `sentrySharedOptions()` は
null を返し、`Backend/internal/observability/sentry.go` も DSN 空なら初期化しない）。

### 段階導入の順序

1. **staging で有効化し、1週間ほど様子を見る**
2. 実際に届いたイベントを開いて、**個人情報やトークンが混ざっていないことを自分の目で確認する**
3. 問題なければ本番を有効化する

**展示会など本番の稼働日の直前に有効化しない。** 新しい外部送信を当日直前に増やさない。

### staging を有効化する

| 対象 | 設定先 | 値 |
|---|---|---|
| ブラウザ側 | GitHub リポジトリシークレット `SENTRY_DSN_FRONTEND_STAGING` | Sentry の Frontend プロジェクトの DSN |
| サーバー側(Backend/RAG) | Terraform 変数 `sentry_dsn`（staging の tfvars） | Sentry の Backend プロジェクトの DSN |

ブラウザ側の DSN は `NEXT_PUBLIC_*` なので**ビルド時にバンドルへ焼き込まれる**。
シークレットを登録したあと、frontend を再ビルドするデプロイが走って初めて有効になる。

サーバー側は staging の `terraform apply` で `.env` に渡り、Backend と rag-review の両方が読む
（同じ `.env` を `env_file` で共有しているため）。

### 本番を有効化する（展示会後）

| 対象 | 設定先 |
|---|---|
| ブラウザ側 | シークレット `SENTRY_DSN_FRONTEND_PROD` |
| サーバー側 | 本番タスク定義の環境変数 `SENTRY_DSN` |

**シークレットは staging と本番で分けてある。** 同じ名前を使うと、staging を有効化した瞬間に
次の本番デプロイでも有効になり、段階導入ができない。

本番のサーバー側だけ注意点がある。`SENTRY_DSN` は ECS タスク定義の `container_definitions` の中
にあり、`terraform apply` で新しいリビジョンは登録されるが、`aws_ecs_service` が `task_definition`
を `ignore_changes` しているため**稼働中のタスクは入れ替わらない**
（`modules/ecs_service_fargate/main.tf`）。反映するには apply 後にデプロイを1回走らせる
（`deployment.yml` は最新リビジョンを取得して image だけ差し替えるので、terraform が入れた
環境変数はそのまま引き継がれる）。frontend/backend に差分が無い場合は
[3.5 の「本番タスク定義へ反映する」](#本番タスク定義へ反映する)と同じ手順で手動反映する。

> かつては `container_definitions` も `ignore_changes` に入っていて、**Terraform に環境変数を
> 足しても永久に本番へ届かなかった**（#1407）。現在は外してあるので、一時的に ignore を外す
> といった作業は不要。

### 送信前に落としているもの

`frontend/lib/sentry.ts` で以下を除去している。追加するときはここも更新すること。

- `Authorization` / `Cookie` / `X-*-Token` などの認証ヘッダー
- **URL のクエリとフラグメント** — `/verify-email?token=` `/company-portal/setup?token=`
  `/auth/callback?user=` にワンタイムトークンやユーザー情報が載るため
- **Referer** — 遷移元のクエリ（＝トークン）が載る
- リクエストボディ（履歴書・チャット本文の混入防止）
- パンくずの URL、および console のパンくずは丸ごと破棄

`sendDefaultPii: false` / `tracesSampleRate: 0`（トレースは送らない＝Sentry の枠を消費しない）。

---

## 3.5 クライアントIPをBFFからBackendへ引き継ぐ (#1407)

Next.js の Route Handler（BFF）が Backend を呼ぶと、Backend から見た送信元は
frontend タスクの出口IP1つに収束する。転送しないと「IP単位」のレート制限
（ログイン 20回/分、パスワードリセット 5回/時、企業情報のゲスト投稿 5回/時、
ゲストAI）が**全利用者合計の上限**として効き、展示会など同時アクセスが増える場面で
無関係な利用者が 429 で締め出される。

### 信頼境界

backend の ALB はインターネットに直結しているため、`X-Client-IP` は誰でも送れる。
無条件に採用するとIP単位の制限を詐称で回避できるので、**BFF と共有するトークンが
一致した場合に限り**採用する。

| 区間 | ヘッダー | 採用条件 |
| --- | --- | --- |
| ブラウザ -> CloudFront/ALB/nginx -> Next.js | `X-Forwarded-For` | 各プロキシが末尾へ直前の送信元を追記する。クライアント指定値は先頭に残る |
| CloudFront -> ALB -> Next.js | `X-Origin-Token` | CloudFront のカスタムオリジンヘッダー。ビューアーが同名ヘッダーを送っても CloudFront が上書きするので詐称できない |
| Next.js -> Backend | `X-Client-IP` + `X-Internal-Token` | `lib/api-proxy.ts` の `clientIpHeaders` が、XFF の末尾から `TRUSTED_PROXY_HOPS` 番目の要素だけを載せる。クライアントが送ってきた `X-Client-IP` は読まない |
| Backend | - | `middleware.GetClientIP` が `BFF_INTERNAL_TOKEN` と定数時間比較し、一致かつIPとして妥当なときだけ採用。それ以外は従来どおり ALB が付けた XFF 末尾へフォールバック |

トークン未設定なら BFF は何も送らず、Backend も何も見ない（従来の挙動のまま）。
シークレット未配布でサイトが落ちないようにするための無効化であって、素通しではない。

#### CloudFront を迂回する経路を数えない

本番の ALB は `0.0.0.0/0` に開いており、`api.shukatsu-ai.jp` から得た同じ ALB の IP へ
frontend の Host/SNI で直接接続できる。この迂回経路で
`X-Forwarded-For: <詐称IP>` を送ると、ALB が実IPを末尾へ足して**要素数2の XFF**が
出来上がり、CloudFront 経由の正規リクエストと段数では区別が付かない。
段数だけで信用すると、詐称した先頭を BFF が内部トークン付きで署名して Backend へ渡し、
IP単位の制限を任意のIPごとに分散できてしまう。

そのため `CLOUDFRONT_ORIGIN_TOKEN` を設定した環境では、CloudFront が付けた
`X-Origin-Token` が一致したときだけ段数を信用する。一致しない（＝ALB 直叩き）場合は
何も転送せず、Backend は BFF の出口IPへフォールバックする。

### 環境変数

| 変数 | 置き場所 | 値 |
| --- | --- | --- |
| `BFF_INTERNAL_TOKEN` | frontend タスクと backend タスクの両方 | 同じランダム文字列。prod は Secrets Manager `soc-app/bff-internal` の `bff_internal_token`、staging は EC2 の共有 `.env` + SSM `/soc-stg/bff-internal-token`（いずれも Terraform の `random_password` が生成。staging は全インスタンスで同一必須） |
| `TRUSTED_PROXY_HOPS` | frontend タスクのみ | frontend の手前にいる**必ず通る**プロキシの段数。prod(CloudFront+ALB)=2、prod で `enable_error_fallback=false`(ALBのみ)=1、staging(ALB+edge nginx)=2、ローカル=未設定(1) |
| `CLOUDFRONT_ORIGIN_TOKEN` | frontend タスクのみ | CloudFront が `X-Origin-Token` として付ける値。Secrets Manager `soc-app/bff-internal` の `cloudfront_origin_token`。CloudFront を迂回できる環境（prod）でのみ設定する |

`TRUSTED_PROXY_HOPS` は経路を1段でも増減させたら必ず合わせる。多く見積もると
クライアントが先頭に詰めた詐称値を拾いうるため、迷ったら小さい値（＝より末尾側）にする。
値が XFF の要素数を超えた場合は推測せず転送しない。

数えてよいのは**成功経路に必ず存在し、かつ迂回できない段**だけ。
staging の CloudFront は Route53 SECONDARY の S3 エラーページ専用（失敗時のみ）なので
段数に数えず、常に 2（ALB + edge nginx。nginx へは ALB の SG からしか届かない）。
prod の CloudFront は迂回できるので、段数に数える代わりに
`CLOUDFRONT_ORIGIN_TOKEN` で経路を認証する。

### 本番タスク定義へ反映する

`environment` / `secrets` は ECS タスク定義の `container_definitions` の中にある。
`terraform apply` で新リビジョンは登録されるが、`aws_ecs_service` は `task_definition` を
`ignore_changes` しているため**稼働中のタスクは入れ替わらない**
（`infra/terraform/modules/ecs_service_fargate/main.tf` の `lifecycle`）。反映するには apply 後に
デプロイを1回走らせる（`deployment.yml` は最新リビジョンを取得して image だけ差し替えるので、
terraform が入れた環境変数はそのまま引き継がれる）。frontend/backend に差分が無い場合は
下記の手順で手動反映する。

`BFF_INTERNAL_TOKEN` は frontend と backend の**両方**に入って初めて機能する。
frontend だけ入れ替えると backend は旧リビジョンのまま期待トークンが空になり、
`GetClientIP` が転送された `X-Client-IP` を全て捨てて frontend の出口IP1つへ
再び集約される。必ず2サービスとも流す。

> **`--task-definition soc-app-<svc>`（リビジョン省略）で update-service してはいけない。**
> リビジョンを省略すると AWS CLI は**最新の ACTIVE リビジョン**＝ terraform が
> `var.backend_image` / `var.frontend_image` から登録したものを選ぶ。本番デプロイ
> (`.github/workflows/deployment.yml`) は `soc-backend:<SHA>` / `soc-frontend:<SHA>` しか
> push せず `latest` を更新しないため、tfvars が `:latest`
> (`infra/terraform/environments/prod/terraform.tfvars.example`) のままだと、
> 稼働中のイメージを古いものへ巻き戻すか、存在しないタグでデプロイを失敗させる。

安全な手順は deployment.yml と同じ「最新リビジョンを取って image だけ差し替えて登録」で、
差し替える先を新しい SHA ではなく**稼働中サービスが今使っている image** にする。

```sh
CLUSTER=soc-app

for svc in backend frontend; do
  container="soc-$svc"   # タスク定義上のコンテナ名(soc-backend / soc-frontend)

  # 1) 稼働中サービスが今使っている image を控える（これを維持するのが目的）
  running_td=$(aws ecs describe-services --cluster "$CLUSTER" --services "$svc" \
    --query 'services[0].taskDefinition' --output text)
  running_image=$(aws ecs describe-task-definition --task-definition "$running_td" \
    --query "taskDefinition.containerDefinitions[?name=='$container'].image | [0]" --output text)
  echo "$svc: 稼働中 $running_td / $running_image"
  case "$running_image" in ""|None) echo "image を取得できない。中止" >&2; exit 1;; esac

  # 2) terraform が登録した最新リビジョン(= 新しい environment/secrets 入り)を取り、
  #    image だけ 1) の値へ戻して新リビジョンとして登録する
  aws ecs describe-task-definition --task-definition "$CLUSTER-$svc" \
    --query 'taskDefinition' \
    | jq --arg c "$container" --arg img "$running_image" \
        'del(.taskDefinitionArn,.revision,.status,.requiresAttributes,.compatibilities,.registeredAt,.registeredBy)
         | .containerDefinitions |= map(if .name == $c then .image = $img else . end)' \
    > "/tmp/$svc-td.json"

  new_td=$(aws ecs register-task-definition --cli-input-json "file:///tmp/$svc-td.json" \
    --query 'taskDefinition.taskDefinitionArn' --output text)

  # 3) 反映前に、狙った環境変数が入っていて image が変わっていないことを確認する
  aws ecs describe-task-definition --task-definition "$new_td" \
    --query "taskDefinition.containerDefinitions[?name=='$container'].{image:image,env:environment[?name=='TRUSTED_PROXY_HOPS'||name=='CLOUDFRONT_ORIGIN_TOKEN'],secrets:secrets[?name=='BFF_INTERNAL_TOKEN']}"

  # 4) リビジョンを明示して適用する（family だけ渡すと 1) の image が上書きされる）
  aws ecs update-service --cluster "$CLUSTER" --service "$svc" --task-definition "$new_td"
done

# 両方が新リビジョンで安定するまで待つ
aws ecs wait services-stable --cluster soc-app --services backend frontend
```

`--force-new-deployment` は不要（`--task-definition` を変えた時点で新しいデプロイが走る）。
本番が停止日（`desired_count=0`）なら update-service だけ打っておけばよく、
`services-stable` は即座に返る。次にスケジューラが起動した時点で新リビジョンが使われる。

### staging の既存インスタンスへ反映する

staging は EC2 上の docker compose で、`.env` を書くのは launch template の user_data
だけ。ASG は `version = "$Latest"` を指定しており、新しい LT 版を作っても**この指定自体は
変化しない**ため instance refresh は起きない。つまり apply しても稼働中のインスタンスは
古い `.env` のままになる。

そのため `deployment.yml` のデプロイ手順で `.env` を更新している。
**デプロイを1回流せば反映される**。

`BFF_INTERNAL_TOKEN` は **staging 全インスタンスで同じ値でなければならない**。
frontend は `BACKEND_URL=https://<backend_domain>`、つまり ALB 配下の任意のインスタンスの
backend を叩くため、インスタンス毎に生成した値だと組み合わせ次第で検証に失敗し、
`X-Client-IP` が捨てられて全利用者が frontend の出口IP1つへ再集約される
（負荷試験で ASG が最大3台まで増えると顕在化する）。

配り方は1系統に揃えてある:

| 対象 | 経路 |
| --- | --- |
| 新規インスタンス | launch template の user_data が `random_password.bff_internal_token` を直接書く |
| 稼働中インスタンス | `deployment.yml` が SSM `/<project>/bff-internal-token` を読んで `.env` を上書き |

SSM パラメータの中身は `random_password.bff_internal_token` と同じ値なので、
どちらの経路でも必ず一致する。SSM を読めなかった場合はデプロイログに `WARN` を出し、
`.env` は変更しない（user_data が書いた正しい値を消さないため）。
`deployment.yml` は **running のインスタンス全台**へ SSH するので、ASG が複数台へ
増えていても配り漏れは出ない。

デプロイを待たずに確認するなら。**トークンの値そのものは表示しない**
（端末のスクロールバック・セッション録画・貼り付けたログに残り、拾った者は
公開 backend ALB へ任意の `X-Client-IP` を送って送信元IPを詐称できる）。
ハッシュの先頭12桁だけを突き合わせる:

```sh
# terraform 側の正解（値は出さずハッシュだけ）
EXPECTED=$(aws ssm get-parameter --name /soc-stg/bff-internal-token \
  --query Parameter.Value --output text \
  | tr -d '\n' | openssl dgst -sha256 | awk '{print substr($NF,1,12)}')
echo "expected: $EXPECTED"

# 全 running インスタンスの実値のハッシュ（すべて expected と一致していること）
aws ec2 describe-instances \
  --filters Name=tag:Name,Values=soc-stg-app Name=instance-state-name,Values=running \
  --query 'Reservations[].Instances[].PublicIpAddress' --output text \
  | tr '\t' '\n' | while read -r ip; do
      printf '%s: ' "$ip"
      ssh ubuntu@"$ip" "sudo sed -n 's/^BFF_INTERNAL_TOKEN=//p' /opt/app/.env \
        | tr -d '\n' | sha256sum | cut -c1-12"
    done
```

### 効いているか確認する

```sh
# 別々の回線（テザリング等）から2回ログインに失敗させ、
# 一方が 429 になっても他方が通ることを見る
curl -i -X POST https://shukatsu-ai.jp/api/company-auth/login \
  -H 'Content-Type: application/json' -d '{"email":"...","password":"..."}'
```

企業情報のゲスト投稿の監査ログ（`company_entry_submissions.source_ip`）も同じ経路でIPを取るため、
転送が効いていれば投稿ごとに異なるIPが記録される。

---

## 4. 障害対応

### 面接レポートが生成されない

**原因候補:**
1. `generateReport` ワーカーがパニック
2. OpenAI APIキー無効

**対処:**
```sh
# ログ確認
docker compose logs app | grep "\[Interview\] Report generation failed"

# 対象セッションIDを特定してAPIを直接叩いて再生成
curl -X POST http://localhost:8080/api/interviews/{sessionID}/send-report \
  -d '{"user_id": 1}'
```

### 職務経歴書レビューが失敗する

**原因候補:**
1. S3接続失敗
2. RAGサービス（rag-review）が停止
3. PDFからテキスト抽出不可

**対処:**
```sh
# RAG + Chroma を確実に起動（ビルド込み）
make rag-up

# 状態確認（vector_store.ok 必須）
curl -s http://localhost:9000/health
curl -s http://localhost:8000/api/v2/heartbeat
curl -s http://localhost:9000/vector/status

# 旧イメージ疑い → 強制 rebuild
make rag-rebuild

# ログ
docker compose logs --tail 80 chroma rag-review
```

### スコアキャリブレーション「サンプル不足」エラー

各カテゴリのサンプルが5件以上必要です。十分なデータが溜まるまでは手動キャリブレーションは不要です。

### 集合知レコメンドが空を返す

**原因:** 類似ユーザー（コサイン類似度 >= 0.85）が見つからない

**対処:**
1. より多くのユーザーが行動ログを蓄積するまで待つ
2. 類似度閾値の引き下げ（`findSimilarUsers` の `threshold` パラメータ）
3. `POST /api/admin/collective-insights/rebuild-summaries` でサマリーを再集計

---

## 5. データベース管理

### デモ企業データのクリーンアップ（Issue #558）

旧 Seed で投入された `example.com` 系のデモ企業は、起動時の `SeedData` 内で `CleanupDemoCompanies` が冪等実行され自動削除されます。既存環境で手動実行する場合:

```sh
cd Backend && go run ./cmd/migrate
```

`cmd/migrate` はスキーマ更新後に `SeedData` を呼び出すため、デモ企業・関連子テーブルも合わせてクリーンアップされます。サーバー起動時（`go run ./cmd/server`）でも同様に実行されます。

**確認クエリ:**

```sql
SELECT id, name, website_url FROM companies
WHERE website_url LIKE '%.example.com%'
   OR website_url = 'https://example.com'
   OR name IN (
     '株式会社テックイノベーション',
     'エンタープライズシステムズ株式会社',
     'クリエイティブラボ株式会社'
   );
-- 0 件であること
```

### バックアップ・DR体制（#620）

本番/staging は RDS（MySQL）を使用。2026-08時点の実設定（`infra/terraform/modules/rds/`、Terraform管理）:

| 項目 | 設定 |
|---|---|
| 保存データ暗号化 | `storage_encrypted = true`（デフォルトAWS管理キー） |
| 自動バックアップ | 有効、保持期間 7日（`BackupRetentionPeriod`） |
| 削除保護 | `deletion_protection = true` |
| PITR（ポイントインタイムリカバリ） | RDS自動バックアップの範囲内（過去7日以内の任意時点） |

S3（履歴書・面接動画）は `infra/terraform/modules/s3/` でサーバーサイド暗号化（SSE）を設定済み。

**復旧手順（PITR、RDS）:**

```sh
# 1. 復旧先の一時DBインスタンスを作成（元インスタンスは変更しない）
aws rds restore-db-instance-to-point-in-time \
  --source-db-instance-identifier soc-app-mysql \
  --target-db-instance-identifier soc-app-mysql-restore-test \
  --restore-time <YYYY-MM-DDThh:mm:ssZ>

# 2. 復旧確認後、アプリのDB接続先を切り替える（DNS/接続文字列の更新が必要）
# 3. 検証済みであれば旧インスタンスを削除、または保持して後日削除
```

**ChromaDB（ベクトルストア）復旧:** 永続ボリューム喪失時は `rag/` の再インデックス処理（企業ドキュメントの再embed）で再構築する。手順は [`chroma-migration.md`](chroma-migration.md) を参照。

> 復旧リハーサル（実際の `restore-db-instance-to-point-in-time` 実行・結果記録）は本番環境への影響を伴うため未実施。実施する場合は本番から隔離された一時インスタンスに対して行い、結果をこのセクションに追記すること。

**ローカル/開発環境の簡易バックアップ:**

```sh
docker compose exec db mysqldump -u root -p app_db > backup_$(date +%Y%m%d).sql
```

**ローカル/開発環境のリストア:**

```sh
docker compose exec -T db mysql -u root -p app_db < backup_YYYYMMDD.sql
```

### よく使うクエリ

```sql
-- ユーザー別スコア確認
SELECT user_id, weight_category, score
FROM user_weight_scores
WHERE user_id = 1
ORDER BY score DESC;

-- 企業別選考通過率
SELECT c.name, uas.status, COUNT(*) as cnt
FROM user_application_statuses uas
JOIN companies c ON c.id = uas.company_id
GROUP BY c.name, uas.status
ORDER BY c.name;

-- 集合知ログ蓄積状況
SELECT action_type, COUNT(*) as cnt
FROM collective_insight_logs
GROUP BY action_type;

-- キャリブレーション履歴
SELECT category, version, weight, pass_rate, correlation, is_active
FROM score_calibration_weights
ORDER BY version DESC, category;
```

---

## 6. 管理画面操作

### 企業プロファイル再計算（#202）

1. `POST /api/admin/profile-recalculation/run` を実行
2. `GET /api/admin/profile-recalculation/history/{companyID}` で更新履歴を確認
3. 問題がある場合は `POST /api/admin/profile-recalculation/{companyID}/rollback` でロールバック

### スコア相関レポート確認（#203）

```sh
GET /api/admin/score-validation/correlation
```

レスポンスの `low_correlated` に含まれるカテゴリは通過率との相関が低いため、質問内容の見直しが推奨されます。

### A/Bテスト設定（#203）

1. バリアント作成:
```sh
POST /api/admin/score-validation/variants
{
  "experiment_name": "phase1_q3_2024",
  "variant_name": "treatment_a",
  "description": "新しい技術志向質問セット",
  "traffic_ratio": 0.5
}
```

2. 結果確認（一定期間後）:
```sh
GET /api/admin/score-validation/variants/results?experiment=phase1_q3_2024
```

### 監査ログ確認

```sh
GET /api/admin/audit-logs
```

全管理操作（企業作成・更新・削除等）が記録されています。

---

## 本番の可観測性（#1497）

2026-09 の本番調査で、**障害を検知・追跡する手段が無い**ことが分かった。

| 仕組み | 当時の状態 |
| --- | --- |
| ALB アクセスログ | 無効。5xx の発生元を特定できない |
| Sentry | **本番のみ未設定**（staging にはある） |
| CloudWatch アラーム | prod / staging とも **0件** |

14日間に ELB 5xx が **50,997件** 出ていたにもかかわらず誰も気付いておらず、
同時期に本番デプロイのマイグレーションも3回連続で失敗していた。

### 入れたもの

**ALB アクセスログ** — `s3://soc-app-alb-logs`、保持30日。
`enable_access_logs` で切り替える。staging は既定の `false` のまま（コスト）。

**Sentry** — `SENTRY_DSN` / `SENTRY_ENVIRONMENT` を backend のタスク定義へ。
`terraform.tfvars` の `sentry_dsn` に値を入れる。空なら `InitSentry` が no-op に
なるので、未設定のままでも起動はする。

**CloudWatch アラーム** — SNS トピック `soc-app-alarms` 経由。
`alarm_email` にアドレスを設定し、届いたメールで **Confirm するまで有効にならない**。

| アラーム | 条件 |
| --- | --- |
| `target-5xx` | アプリの5xxが5分で5件超 |
| `target-latency` | p95応答が3秒超（2回連続） |
| `rds-free-storage` | 空き2GiB未満 |
| `rds-cpu` | CPU 80%超が10分 |

### 設計上の要点: 計画停止中に鳴らさない

本番は「指定日のみ終日起動」で、**稼働していない時間の方が長い**。
停止中に鳴るアラームを置くと通知が無視される状態になり、結局いまと同じになる。

- `treat_missing_data = "notBreaching"`（データが無い＝停止中は正常扱い）
- **停止中でも出る `HTTPCode_ELB_5XX` は対象にしない**。これはターゲット全滅時に
  ALB が返す503で、計画停止中は常時発生する
- 稼働していないと発生しない指標だけを見る

### 停止中の見え方

frontend は CloudFront 経由で、ALB が 500/502/503/504 を返すと
`s3://soc-app-errors-production/service-unavailable.html` へフェイルオーバーする
（`enable_error_fallback`、既定 `true`）。学生には「接続できません」の案内が出る。

API (`api.shukatsu-ai.jp`) は JSON を返す前提のため対象外で、素の503になる。
