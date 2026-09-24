# Redis / ジョブキュー運用メモ（#617）

## 環境変数

| 変数 | 説明 |
|------|------|
| `REDIS_URL` | 例 `redis://localhost:6379/0`。未設定時はインメモリレート制限 + `go func` / in-process channel |

## ローカル

```bash
docker compose up -d redis
# Backend/.env に REDIS_URL=redis://localhost:6379/0
```

compose の `backend` サービスは `redis` に依存し、既定で `REDIS_URL=redis://redis:6379/0`。

## 配置方針

- Redis は **Backend と別コンテナ**（同梱しない）
- staging/prod は ElastiCache または別 ECS サービス推奨（サイドカー同一タスクは非推奨）

## ジョブ

| タスク | リトライ | 備考 |
|--------|----------|------|
| `email:*` | 5 | critical キュー |
| `interview:report` | 3 | default キュー |

失敗はログ `[queue] task failed`。asynq の archive（DLQ）に保持。

`interview:report` は発話が0件のセッションで `ErrNoUtterances` を返し、リトライ対象になる（#1476）。
発話保存がネットワーク瞬断などで欠落した場合に、空レポートを「正常終了」として確定させないため。
リトライし切っても0件ならレポートは作らない（フロントのポーリングがタイムアウトし、生成失敗として見える）。

### 未生成レポートの再生成（#1476）

リトライを使い切る／フォールバックの channel worker で失敗する／`jobCh` が満杯で捨てられる、
のいずれでもレポート生成ジョブは失われる。`POST /api/interviews/:id/finish` は終了済みセッションを
再キューしない（#1019 の冪等性）ため、回復経路は次の1本だけ。

`POST /api/interviews/:id/report/regenerate`

- 終了済み（`status = finished`）かつレポート未生成のときだけ再投入し、`{"queued": true}` を返す
- 既にレポートがあれば何もしない（`{"queued": false}`）。無条件に再投入すると LLM 費用が二重に掛かり、生成中のレポートを上書きする
- 未終了セッションは 400、他人のセッションは 403
- フロントはレポート画面の「再試行」ボタン（ポーリングのタイムアウト／失敗後）から叩く

## フォールバック

Redis 不通時:

- レート制限: フェイルオープン（許可）
- ジョブ: エンキュー失敗時は従来の `go func` / channel にフォールバック
