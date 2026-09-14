# RAG サービス詳細

SOC AI Agent の RAG（Retrieval-Augmented Generation）サービスは Python / FastAPI で実装されており、職務経歴書レビューと企業情報収集を担当します。

---

## ベクトルストア（#573）

| 項目 | 内容 |
|------|------|
| 既定 | Docker Compose の `chroma` サービス（`chromadb/chroma:0.6.3`） |
| RAG 接続 | `CHROMA_HOST` / `CHROMA_PORT` → `HttpClient` |
| フォールバック | `CHROMA_HOST` 未設定時は `PersistentClient`（ローカル開発・単体テスト） |
| 永続化 | compose named volume `chroma_data`（`chroma` 再作成でも保持） |
| コレクション | `company_context` / `interview_hints` / `es_review`（企業メタデータ付き） |
| 旧データ移行 | [chroma-migration.md](./chroma-migration.md)（#585） |
| 設計 | `docs/design/vector-db.md` |

---

## 概要

```
クライアント（Backend Go）
       │ HTTP POST
       ▼
┌──────────────────────────────────────────┐
│  FastAPI RAG（Port 9000）                │
│                                          │
│  ┌────────────────┐   ┌───────────────┐ │
│  │ Chroma Server  │   │ OpenAI Web    │ │
│  │ (ベクトルDB)    │   │ Search        │ │
│  └────────────────┘   └───────────────┘ │
│           │                   │         │
│           └─────────┬─────────┘         │
│                     ▼                   │
│             LLM（GPT-4o）によるレビュー生成│
└──────────────────────────────────────────┘
```

---

## エンドポイント一覧

| メソッド | パス | 概要 |
|---------|------|------|
| POST | `/resume/review` | 職務経歴書レビュー（同期） |
| POST | `/resume/review/stream` | 職務経歴書レビュー（ストリーミング） |
| POST | `/company/hints` | 企業面接ヒント収集 |
| POST | `/es/review` | エントリーシートレビュー |
| GET | `/health` | ヘルスチェック（Chroma 接続含む） |
| GET | `/healthz` | ヘルスチェック（Chroma 失敗時 503） |
| POST | `/company/context` | Backend からの企業コンテキスト書き込み |
| GET | `/vector/status` | ベクトルインデックス状況 |
| POST | `/vector/reembed` | 企業ベクトル削除・再埋め込み |

## 内部サービス認証（#615）

`/health` / `/healthz` を除く全エンドポイントは、Backend からの内部認証ヘッダー `X-Internal-Token` を要求します。

- トークンは環境変数 `RAG_INTERNAL_TOKEN` で設定し、**Backend と RAG で同一の値**を使う（docker compose では両サービスが `Backend/.env` を読むため1箇所の設定で足りる）
- `RAG_INTERNAL_TOKEN` 未設定時はフェイルクローズ: ヘルスチェック以外の全リクエストを 503 で拒否する
- トークン不一致は 401
- 本番のトークンは `openssl rand -hex 32` などで生成する
- Backend 側は `internal/ragclient.SetAuthHeader` がリクエストへ自動付与する（RAG 呼び出しを追加する際は必ずこれを通すこと）

```bash
# 手動でエンドポイントを叩く場合
curl -H "X-Internal-Token: $RAG_INTERNAL_TOKEN" http://localhost:9000/vector/status
```

### `/resume/review` リクエスト例

```json
{
  "company_name": "株式会社Example",
  "job_title": "バックエンドエンジニア",
  "resume_text": "職務経歴...",
  "use_deep_research": false
}
```

### `/resume/review` レスポンス例

```json
{
  "review": "【強み】...\n【改善点】...",
  "context_source": "web_search",
  "retrieved_docs": ["企業情報テキスト..."]
}
```

`context_source` の値:

| 値 | 意味 |
|----|------|
| `deep_research` | OpenAI Deep Research（o3-deep-research）を使用 |
| `web_search` | OpenAI Web Search（gpt-4o-search-preview）を使用 |
| `cache` | ChromaDB のキャッシュを使用（Web 検索なし） |

---

## ChromaDB キャッシュ戦略

### キャッシュの仕組み

```
1. キャッシュキー生成
   cache_key = "{company_name}::{job_title}"

2. ChromaDB でベクトル検索
   ├── ヒット → 類似度順で最大 5 件取得（Web 検索スキップ）
   └── ミス → Web Search パイプラインを実行 → ChromaDB に保存
```

### 設定

```env
# 独立 Chroma（推奨）
CHROMA_HOST=chroma
CHROMA_PORT=8000

# CHROMA_HOST 未設定時のみ PersistentClient 用
RAG_CHROMA_DATA_DIR=/app/chroma_db
```

### キャッシュのリセット

HttpClient 構成では RAG コンテナ内の `/app/chroma_db` 削除では消えません。`/vector/reembed` または Chroma 側コレクション削除を使います。ロールバックは [chroma-migration.md](./chroma-migration.md) を参照。

---

## OpenAI Web Search パイプライン

クエリ生成 → 並列検索 → ドメイン信頼度スコアリング → LLM 要約 の順で実行されます。

```
1. クエリ生成（_generate_search_queries）
   │ 企業名・職種から 3〜5 個の検索クエリを自動生成
   │
   ▼
2. 並列 Web 検索（_web_search_openai × N）
   │ ThreadPoolExecutor で並列実行
   │ 使用モデル: gpt-4o-search-preview
   │
   ▼
3. ドメイン信頼度スコアリング（rank_results_by_domain_trust）
   │ 公式ドメイン・ニュースサイト等を優先
   │
   ▼
4. LLM 要約（_summarize_for_hiring）
   │ 採用観点での要約生成
   │
   ▼
5. ChromaDB 保存 + 検索ログ記録（JSONL）
```

---

## Deep Research モード

### 概要

OpenAI の `o3-deep-research` モデルを使用した高精度な企業調査機能です。
通常の Web Search より詳細な情報を取得できますが、レスポンスタイムが大きく増加します。

### 切り替え方法

**環境変数で制御:**

```env
# Deep Research を有効化（デフォルト: true）
RAG_USE_DEEP_RESEARCH=true

# Web Search のみを使用する場合
RAG_USE_DEEP_RESEARCH=false
```

**リクエストボディで制御:**

```json
{
  "company_name": "...",
  "job_title": "...",
  "resume_text": "...",
  "use_deep_research": true
}
```

### 動作フロー

```
use_deep_research=true の場合:
  1. Deep Research（o3-deep-research）を試みる
  2. 成功 → context_source="deep_research" で返却
  3. 失敗（モデル未対応等）→ Web Search にフォールバック

use_deep_research=false の場合:
  1. ChromaDB キャッシュを確認
  2. ヒット → context_source="cache" で返却
  3. ミス → Web Search パイプラインを実行
```

### 注意事項

- Deep Research は `openai>=1.66` が必要です（`constraints.txt` で管理）
- タイムアウトが設定されており、超過した場合は Web Search にフォールバックします

---

## 検索ログ（JSONL）の活用

### ログ保存先

```
/app/search_logs/search_log.jsonl  （コンテナ内）
```

### ログ形式

```jsonl
{"company_name": "...", "job_title": "...", "queries": ["..."], "raw_results": ["..."], "summary": "...", "timestamp": "..."}
```

### ファインチューニングデータとしての活用

検索ログは LLM のファインチューニング用データとして収集されています。

```sh
# トレーニングデータのエクスポート
cd rag
python3 export_training_data.py
```

```sh
# REST API でエクスポート
GET /training/export
```

エクスポートされたデータは `training/` ディレクトリ以下に保存されます。
詳細は [`rag/FINETUNE_README.md`](../../rag/FINETUNE_README.md) を参照してください。

---

## 構成ファイル

| ファイル | 説明 |
|---------|------|
| `rag/main.py` | FastAPI メインアプリケーション |
| `rag/training_api.py` | ファインチューニングデータ出力 API |
| `rag/constraints.txt` | バージョン制約ファイル（`-c` で渡す。単体で `-r` に渡さない） |
| `rag/requirements.txt` | 直接依存の宣言（主インストール元。全件に上下限つき。`test_requirements_declaration_is_bounded` が強制する） |
| `rag/export_training_data.py` | ログからトレーニングデータを生成 |

---

## ローカル開発・デバッグ

```sh
# RAG サービスのみ起動
docker compose --profile rag up -d rag-review

# ログ確認
docker compose logs -f rag-review

# Python 環境での直接起動（デバッグ時）
cd rag
pip install -r requirements.txt -c constraints.txt
LOG_LEVEL=DEBUG python3 main.py
```

### テスト

```sh
cd rag
python3 -m pytest tests/ -v
```

---

## 関連ドキュメント

- [システム概要](./overview.md) — 全体アーキテクチャ
- [API リファレンス](./api-reference.md) — バックエンド API 一覧
- [Getting Started](./getting-started.md) — 環境構築手順


## Web検索のコスト（#1124）

OpenAI の `web_search` ツールは、検索結果が固定トークンとして課金される。
本文の長さに関係なく1コールの入力トークンが大きくなるため、
**コストは「1コールの重さ × コール回数」でほぼ決まる**。

本番相当DB（2026-08-13以降, 2,981コール）の実測:

| model | コール数 | 入力トークン | 1コール平均 | 全体比 |
| --- | ---: | ---: | ---: | ---: |
| gpt-5-search-api | 323 | 9,816,772 | 30,392 | 約9割 |
| gpt-4o-mini | 2,206 | 8,926,489 | 4,046 | 約1割 |

検索1コールが通常の推論7.5倍の入力トークンを使っている。

### 調整ノブ

| env | 既定 | 範囲 | 効果 |
| --- | --- | --- | --- |
| `OPENAI_WEB_SEARCH_CONTEXT_SIZE` | `medium` | low / medium / high | 1コールの重さ。以前は `high` 固定だった |
| `OPENAI_WEB_SEARCH_MAX_QUERIES` | `4` | 1〜10 | 1企業あたりのコール回数。以前は 5 固定 |
| `OPENAI_CHAT_MODEL` | `gpt-4o-mini` | - | 要約・クエリ生成。以前は `gpt-4o` 既定（入力単価16.7倍） |
| `OPENAI_HINTS_PARSE_MODEL` | `gpt-4o-mini` | - | JSON構造化抽出。以前は `gpt-4o` 既定 |

不正な値は既定に倒す（設定ミスでリクエストを止めない）。
品質劣化が出た場合は env を戻すだけでよく、デプロイは不要。

反映後は `api_call_logs` の model 別内訳で削減幅を確認すること。
なお 2026-09-14 より前の記録はコストが過大なので、比較は同日以降で行う
（`docs/wiki/search-provider-cost.md` 参照）。
