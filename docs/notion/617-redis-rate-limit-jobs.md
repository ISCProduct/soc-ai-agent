# #617 Redis レート制限・永続ジョブキュー

> **Issue:** https://github.com/ISCProduct/soc-ai-agent/issues/617  
> **Backlog:** SOCAIAGENT-62  
> **最終更新:** 2026-08-06  
> **リポジトリ:** `docs/architecture/redis-horizontal-scale-plan-617.md` / `docs/wiki/redis-jobs.md`

---

## 1. 概要

| 項目 | 内容 |
|------|------|
| 背景 | Backend が単一インスタンス前提で、水平スケールできない |
| 問題 | レート制限が `sync.Map` インメモリ / 非同期が `go func` で再起動時に消失 |
| ゴール | Redis ベースのレート制限 + 永続ジョブキュー（asynq） |
| 受け入れ | 複数インスタンスでも制限が効く / 重要ジョブが再起動を跨ぐ / 失敗が観測できる |

### 実装ステータス（2026-08-06）

- [x] Phase 0: compose redis + `REDIS_URL` + 接続ヘルパ
- [x] Phase 1: Redis レート制限（Login / PasswordReset）
- [x] Phase 2: asynq クライアント・サーバー
- [x] Phase 3: メール送信・面接レポートのキュー移行
- [ ] Phase 4: 観測（asynqmon 等）
- [ ] Phase 5: staging/prod の Redis（ElastiCache / 別 ECS）

ブランチ: `feat/617-redis-rate-limit-job-queue`

---

## 2. 現状コード（主な箇所）

- レート制限: `Backend/internal/middleware/rate_limiter.go`
- 消失しうる非同期例:
  - 認証メール送信（`auth_register.go` / `auth_session.go`）
  - 面接レポート生成（`interview_report.go` の in-process `jobCh`）
  - 面接動画アップロード後処理（`interview_controller.go`）
  - カレンダー同期（`schedule_service.go`）
  - GitHub sync / コスト通知 / RAG push 等

---

## 3. Redis 配置方針

### 結論

| 環境 | Redis の置き方 | 理由 |
|------|----------------|------|
| ローカル / docker compose | **同居（別コンテナ）** | 手軽。`redis:7-alpine` を compose に追加 |
| staging（AWS ECS） | **別 ECS サービス or ElastiCache** | 永続・フェイルオーバーと相性が良い |
| 本番（AWS・指定起動） | **ElastiCache 推奨** | ジョブキューはプロセス寿命より長く生きる必要がある |

### やってはいけないこと

```
[OK]  docker compose: backend + redis（別サービス）
[OK]  ECS: redis を別タスク/サービス（desired=1、永続ボリューム）
[NG]  backend イメージに redis-server を入れた単一コンテナ
[NG]  ECS タスク定義で backend と redis を同一タスクに詰め込み
```

**Backend コンテナ内に Redis バイナリ同梱はしない。** 1 プロセス障害で Redis も落ち、水平スケール時にインスタンス数ぶん Redis が分裂する。

### コスト感（staging 常時前提）

| 選択肢 | 月額目安 | 備考 |
|--------|----------|------|
| compose 同居 redis | ≒ ¥0（ローカル） | 開発必須 |
| ECS 上 redis コンテナ 1 | 追加ほぼなし〜数千円 | AOF 永続推奨 |
| ElastiCache cache.t4g.micro | おおよそ ¥2,000〜4,000 | 運用が楽。本番向き |

---

## 4. 技術選定

| 領域 | 選定 | 理由 |
|------|------|------|
| Redis クライアント | `github.com/redis/go-redis/v9` | 標準的 |
| レート制限 | Redis + sliding window（Lua） | 既存ウィンドウ値を踏襲 |
| ジョブキュー | **asynq** | Go 親和性、リトライ・DLQ |
| フォールバック | `REDIS_URL` 未設定時はインメモリ + `go func` | 段階移行 |

---

## 5. ディレクトリ構成

```
Backend/
  internal/middleware/rate_limiter.go
  internal/middleware/rate_limiter_redis.go
  internal/infrastructure/redisx/client.go
  internal/queue/                    # asynq client/server
  cmd/server/main.go                 # redis + asynq 起動
docker-compose.yml                   # redis サービス
docs/wiki/redis-jobs.md
```

---

## 6. 運用メモ

### 環境変数

| 変数 | 説明 |
|------|------|
| `REDIS_URL` | 例 `redis://localhost:6379/0`。未設定時はインメモリ + `go func` |

### ローカル起動

```bash
docker compose up -d redis
# Backend/.env に REDIS_URL=redis://localhost:6379/0
```

compose の `backend` は `redis` に依存。既定 `REDIS_URL=redis://redis:6379/0`。

### ジョブ

| タスク | リトライ | 備考 |
|--------|----------|------|
| `email:*` | 5 | critical キュー |
| `interview:report` | 3 | default キュー |

失敗はログ `[queue] task failed`。asynq の archive（DLQ）に保持。

### フォールバック（Redis 不通時）

- レート制限: フェイルオープン（許可）
- ジョブ: エンキュー失敗時は従来の `go func` / channel にフォールバック

---

## 7. リトライ / DLQ 方針

| 項目 | 案 |
|------|-----|
| 最大リトライ | メール 5 / レポート 3 / 動画 5 |
| backoff | asynq デフォルト指数 |
| DLQ | asynq archived。ログに `task archived` |
| 手動再実行 | asynqmon または管理 API（後続） |

---

## 8. 受け入れ条件

- [x] 複数 Backend 相当でも Login/PWリセット制限が共有される
- [x] レポート生成・認証メールが API プロセス再起動後も完遂される（Redis 利用時）
- [x] 失敗がログで追える
- [x] `REDIS_URL` なしのローカルは現行互換

---

## 9. 残タスク（後続）

| 優先 | ジョブ | 現行 |
|------|--------|------|
| P1 | 面接動画の後処理 | controller 内 `go func` |
| P2 | Google カレンダー同期 | `go calendarSync.*` |
| P3 | GitHub sync / RAG push / コスト通知 | 各 `go func` |
| — | Terraform Redis（staging/prod） | 未着手 |
| — | asynqmon / 観測ダッシュボード | 未着手 |

---

## 10. Out of scope（初期）

- Redis Cluster / Sentinel 本格 HA
- 全 `go func` の機械的移行
- FE 変更
- マルチテナント向けキュー分離
