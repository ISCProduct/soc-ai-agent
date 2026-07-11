# 設計: 本番向けベクトルDB導入 (#573)

## 1. 現状

| 項目 | 内容 |
|------|------|
| 実装 | `rag/main.py` の `chromadb.PersistentClient`（ローカルディスク） |
| 埋め込み | OpenAI `text-embedding-3-small`（環境変数で変更可） |
| コレクション | キャッシュキーごと（例: `hints::{company}::{role}`）に1コレクション |
| 課題 | 複数 RAG インスタンスで共有不可、再デプロイで消失、職種別キーで WebSearch 再実行（#510） |

## 2. 製品比較と選定

| 候補 | 移行コスト | 運用 | 判定 |
|------|-----------|------|------|
| **Chroma Server** | 低（同一 SDK） | Docker / ボリューム永続 | **採用（Phase 1）** |
| pgvector | 高（Postgres 追加） | SQL 一体で強い | 将来候補 |
| Qdrant / Weaviate | 中 | 機能豊富 | 将来候補 |
| マネージド Vector Store | 中 | 運用負荷低 | コスト見合いで再評価 |

**推奨:** まず **Chroma Server** を独立コンテナとして導入し、RAG は `HttpClient` で接続する。  
既存 `chromadb` 依存を維持でき、破壊的変更を最小化できる。

## 3. コレクション設計

用途横断で固定コレクションを使う（キー単位コレクションは廃止方向）。

| コレクション | doc_type | 用途 |
|--------------|----------|------|
| `company_context` | `resume_review` / `company_research` | 履歴書レビュー・企業調査 |
| `interview_hints` | `interview_hints` | 面接ヒント |
| `es_review` | `es_review` | ES 添削コンテキスト |

### メタデータ（必須）

| キー | 型 | 説明 |
|------|-----|------|
| `company` | string | 正規化企業名 |
| `role` | string | 職種（空なら `general`） |
| `doc_type` | string | 上記 |
| `source` | string | `web_search` / `deep_research` / `job_fetch` / `manual` 等 |
| `fetched_at` | string (ISO8601) | 取得日時 |
| `cache_key` | string | 旧キー互換用 |

### クエリ方針（#510 対応）

1. `interview_hints` で `company` + `role` が一致するドキュメントを検索
2. ミス時は同一 `company` の任意 role を再利用（WebSearch 抑制）
3. それでも無ければ WebSearch → upsert

## 4. 埋め込み・コスト

- モデル: `text-embedding-3-small`（次元 1536）
- 1 ドキュメントあたり概算: 数百〜数千トークン
- TTL: `RAG_SEARCH_CACHE_TTL_SECONDS`（デフォルト 86400）。メタデータ `fetched_at` で期限切れ判定可能（Phase 2 でフィルタ実装）

## 5. インフラ

### ローカル

```yaml
chroma:
  image: chromadb/chroma:0.6.3
  ports: ["8000:8000"]
  volumes: [chroma_data:/chroma/chroma]
```

RAG 環境変数:

- `CHROMA_HOST=chroma`（未設定時は従来の PersistentClient フォールバック）
- `CHROMA_PORT=8000`

### 本番（方針）

- ECS タスク or サイドカーで Chroma Server
- EBS / EFS で `/chroma/chroma` を永続化
- ヘルスチェック: `GET http://chroma:8000/api/v1/heartbeat`（バージョンによりパス差あり）
- バックアップ: ボリュームスナップショット

## 6. 移行方針

- **コールドスタート許容:** 旧 PersistentClient のローカルデータを自動移行しない
- `CHROMA_HOST` 未設定環境では PersistentClient を維持（開発・単体テスト）
- Phase 3 で Backend 職種 embedding / チャット関連度との統合を検討

## 7. 受け入れとの対応

| 条件 | 対応 |
|------|------|
| compose でベクトルDB起動 | `chroma` サービス |
| RAG が新DB経由 | `HttpClient` + 固定コレクション |
| WebSearch 削減 | 企業単位フォールバック検索 |
| source / fetched_at | upsert メタデータ |
| 設計ドキュメント | 本ファイル |
| ラウンドトリップテスト | `rag/tests/test_vector_store.py` |
