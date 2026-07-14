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

### ヘルスチェック確認

```sh
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

### マイグレーション

GORMの `AutoMigrate` はサーバー起動時に自動実行されます。手動実行する場合:

```sh
cd Backend && go run ./cmd/migrate
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
- `REALTIME_MONTHLY_ALERT_THRESHOLD_USD=200` を超えるとメール/Slackアラート
- `REALTIME_ALERT_EMAILS` / `REALTIME_ALERT_SLACK_WEBHOOK_URL` で通知先設定

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

### バックアップ

```sh
docker compose exec db mysqldump -u root -p app_db > backup_$(date +%Y%m%d).sql
```

### リストア

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
