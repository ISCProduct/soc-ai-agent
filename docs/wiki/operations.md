# 運用手順書

## 目次

1. [デプロイ手順](#1-デプロイ手順)
2. [定期バッチ作業](#2-定期バッチ作業)
3. [監視項目](#3-監視項目)
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
docker compose --profile rag logs --tail 80 chroma rag-review
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
