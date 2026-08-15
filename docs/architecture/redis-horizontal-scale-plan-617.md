# 実装計画: #617 水平スケール対応（Redis レート制限・永続ジョブキュー）

最終更新: 2026-08-06  
Issue: https://github.com/ISCProduct/soc-ai-agent/issues/617  
Backlog: SOCAIAGENT-62

## 1. Issue 要約

| 項目 | 内容 |
|------|------|
| 背景 | Backend が単一インスタンス前提で、水平スケールできない |
| 問題 | レート制限が `sync.Map` インメモリ / 非同期が `go func` で再起動時に消失 |
| ゴール | Redis ベースのレート制限 + 永続ジョブキュー（asynq 等） |
| 受け入れ | 複数インスタンスでも制限が効く / 重要ジョブが再起動を跨ぐ / 失敗が観測できる |

### 現状コード（主な箇所）

- レート制限: `Backend/internal/middleware/rate_limiter.go`（Login / PasswordReset）
- 消失しうる非同期例:
  - 認証メール送信（`auth_register.go` / `auth_session.go`）
  - 面接レポート生成（`interview_report.go` の in-process `jobCh`）
  - 面接動画アップロード後処理（`interview_controller.go`）
  - カレンダー同期（`schedule_service.go`）
  - GitHub sync / コスト通知 / RAG push 等

---

## 2. Redis をコンテナ同居させるか

### 結論（推奨）

| 環境 | Redis の置き方 | 理由 |
|------|----------------|------|
| **ローカル / docker compose** | **同居（別コンテナ）** | 手軽。`redis:7-alpine` を compose に追加。Backend と同ネットワーク |
| **staging（AWS ECS）** | **同居（同一タスク or 同一サービス群のサイドカー相当）は非推奨**。**別 ECS サービス or ElastiCache** | データ永続・フェイルオーバー・タスク再配置と相性が悪い |
| **本番（AWS・指定起動）** | **ElastiCache（単一ノード可）または managed Redis** を第一候補。コスト次第で **別 ECS サービス + EBS/EFS ボリューム** | ジョブキューはプロセス寿命より長く生きる必要がある |

### 「Backend コンテナ内に Redis バイナリ同梱」はしない

- 1 プロセス障害で Redis も落ちる
- 水平スケール時にインスタンス数ぶん Redis が分裂する（レート制限が再び効かない）
- 運用（永続化・バックアップ）が困難

### 「compose / ECS で redis サービスを別コンテナ」は可（ローカル〜検証）

```text
[OK]  docker compose: backend + redis（別サービス）
[OK]  ECS: redis を別タスク/サービス（desired=1、永続ボリューム）
[NG]  backend イメージに redis-server を入れた単一コンテナ
[NG]  ECS タスク定義で backend と redis を同一タスクに詰め込み、タスク置換でキュー消失
```

同一 ECS タスクにサイドカーで Redis を入れる案は、**タスク入れ替えのたびにキュー・レート制限状態が消える**ため、#617 の「永続ジョブ」要件と矛盾しやすい。検証専用の短命環境以外は避ける。

### コスト感（staging 常時前提）

| 選択肢 | 月額目安 | 備考 |
|--------|----------|------|
| compose 同居 redis | ≒ ¥0（ローカル） | 開発必須 |
| ECS 上 redis コンテナ 1 | EC2 同居なら追加ほぼなし / 別小さめタスクなら数千円 | AOF 永続推奨 |
| ElastiCache cache.t4g.micro | おおよそ ¥2,000〜4,000 前後 | 運用が楽。本番向き |

**staging 初期:** compose で同居コンテナ → ECS staging では **別サービス Redis（永続）または ElastiCache micro**。  
**prod:** ElastiCache 推奨（指定日のみ ON なら、停止運用とセットで設計）。

---

## 3. 技術選定

| 領域 | 選定 | 理由 |
|------|------|------|
| Redis クライアント | `github.com/redis/go-redis/v9` | 標準的 |
| レート制限 | Redis + sliding window（ZSET or INCR+TTL）。既存ウィンドウ値を踏襲 | Issue 方針どおり。ライブラリは自前 or `redis_rate` 等 |
| ジョブキュー | **asynq**（Redis ベース） | Go 親和性、リトライ・DLQ・ダッシュボード、Issue 記載例と一致 |
| フォールバック | `REDIS_URL` 未設定時は現行インメモリ + `go func`（開発容易性） | 段階移行。本番/staging では Redis 必須にしてもよい |

---

## 4. フェーズ計画

### Phase 0 — 基盤（0.5日）

- [ ] `docker-compose.yml` に `redis` サービス追加（永続 volume、`6379`）
- [ ] 環境変数: `REDIS_URL=redis://redis:6379/0`（`.env.example` 追記）
- [ ] Backend 起動時に Redis ping。失敗時の挙動を定義（fail-open 開発 / fail-closed staging）
- [ ] 接続ヘルパ `internal/infrastructure/redis`（仮）

### Phase 1 — レート制限の Redis 化（1日）

- [ ] `RateLimiter` インターフェース化（`Allow(key string) bool`）
- [ ] 実装: `memoryRateLimiter`（現行）/ `redisRateLimiter`
- [ ] Login / PasswordReset を差し替え（閾値は現行維持: 1分20回 / 1時間5回）
- [ ] 複数プロセスを模したテスト（同一 Redis に対する並行 Allow）
- [ ] 既存 `rate_limiter_test.go` をインターフェース向けに更新

**完了条件:** compose で backend×2（または並列テスト）でも制限が共有される

### Phase 2 — asynq ワーカー導入（1〜2日）

- [ ] asynq Server / Client を DI（`cmd/server` で API と同居起動、または `cmd/worker` 分離）
  - **初期は API プロセス内で worker も起動**（運用単純）
  - 将来スケール時に `cmd/worker` を分離可能な構造にする
- [ ] 共通: リトライ回数・backoff・Dead letter（asynq の Archive）方針をコードと docs に記載
- [ ] 構造化ログ: `task_type`, `task_id`, `retry`, `error`

### Phase 3 — 重要ジョブの移行（優先順）（2日）

| 優先 | ジョブ | 現行 | 移行理由 |
|------|--------|------|----------|
| P0 | メール送信（登録確認・再認証・PWリセット等） | `go emailService.Send*` | ユーザー到達性。再送必要 |
| P0 | 面接レポート生成 | in-memory `jobCh` | 再起動でロストしやすい |
| P1 | 面接動画の後処理（S3 アップロード完了後） | controller 内 `go func` | 時間が長く失敗しやすい |
| P2 | Google カレンダー同期 | `go calendarSync.*` | 失敗しても本処理は成功扱いだが再試行したい |
| P3 | GitHub sync / RAG push / コスト通知 | 各 `go func` | 後続でも可 |

各ジョブは payload を JSON 化し、冪等キー（例: `user_id+token` / `session_id`）を意識する。

### Phase 4 — 観測（0.5日）

- [ ] 失敗タスクをログに必ず出す
- [ ] （任意）asynqmon を compose profile で追加、または簡易 admin メトリクス
- [ ] 受け入れ用の手動手順（強制失敗→リトライ→成功）を README に記載

### Phase 5 — infra（staging/prod）（1日・後続可）

- [ ] Terraform: Redis（ElastiCache or ECS service）モジュール草案
- [ ] staging 常時: Redis 常時
- [ ] prod 指定起動: Redis も ECS/ElastiCache と連動して start/stop するか方針決定
- [ ] Backend タスクに `REDIS_URL` 注入

---

## 5. ディレクトリ / 変更イメージ

```text
Backend/
  internal/middleware/rate_limiter.go     # インターフェース + memory
  internal/middleware/rate_limiter_redis.go
  internal/queue/                         # asynq client/server, task types
  internal/queue/tasks/                   # email, interview_report, ...
  cmd/server/main.go                      # redis + asynq 起動
  cmd/worker/main.go                      # （任意・Phase 後半）
docker-compose.yml                        # redis サービス
Backend/.env.example                      # REDIS_URL
docs/wiki/redis-jobs.md                   # リトライ・DLQ・運用
```

---

## 6. リトライ / DLQ 方針（案）

| 項目 | 案 |
|------|-----|
| 最大リトライ | メール 5 / レポート 3 / 動画 5 |
| backoff | asynq デフォルト指数 or 固定+ジッタ |
| DLQ | asynq archived。ログに `task archived` |
| 手動再実行 | asynqmon または管理 API（後続） |
| タイムアウト | タスク毎に context deadline |

---

## 7. 受け入れ条件（Issue 対応）

- [ ] 複数 Backend 相当でも Login/PWリセット制限が共有される
- [ ] Redis 再起動後も（AOF/永続時）キュー上の未完了ジョブを処理できる ※メモリのみ構成なら要注記
- [ ] レポート生成・認証メールが API プロセス再起動後も完遂される
- [ ] 失敗がログで追える
- [ ] `REDIS_URL` なしのローカルは現行互換（または明示的にスキップ）

---

## 8. Out of scope（初期）

- Redis Cluster / Sentinel 本格 HA
- 全 `go func` の機械的移行（P3 は後続）
- FE 変更
- マルチテナント向けキュー分離

---

## 9. 推奨実装順（すぐ着手する場合）

1. compose に **redis 別コンテナ**追加  
2. レート制限 Redis 化 + テスト  
3. asynq + メール / 面接レポート移行  
4. staging の Redis 配置（別サービス or ElastiCache）を infra 計画に接続  

---

## 10. Redis 同居判断のまとめ（再掲）

- **ローカル: 同居（別コンテナ）でよい**  
- **Backend と同じコンテナに入れる: だめ**  
- **ECS 同一タスクサイドカー: 永続ジョブ用途では非推奨**  
- **staging/prod: 別サービスまたは ElastiCache**  

この方針で Phase 0 から実装に進めてよければ指示ください。
