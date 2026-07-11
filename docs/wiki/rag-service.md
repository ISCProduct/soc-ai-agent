# RAG サービス詳細

SOC AI Agent の RAG（Retrieval-Augmented Generation）サービスは Python / FastAPI で実装されており、職務経歴書レビューと企業情報収集を担当します。

---

## ベクトルストア（#573）

| 項目 | 内容 |
|------|------|
| 既定 | Docker Compose の `chroma` サービス（`chromadb/chroma:0.6.3`） |
| RAG 接続 | `CHROMA_HOST` / `CHROMA_PORT` → `HttpClient` |
| フォールバック | `CHROMA_HOST` 未設定時は `PersistentClient`（ローカル開発・単体テスト） |
| コレクション | `company_context` / `interview_hints` / `es_review`（企業メタデータ付き） |
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
| GET | `/health` | ヘルスチェック |
| GET | `/healthz` | ヘルスチェック（簡易） |

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
# ChromaDB データディレクトリ（デフォルト: /app/chroma_db）
RAG_CHROMA_DATA_DIR=/app/chroma_db
```

### キャッシュのリセット

```sh
# Docker Compose 環境の場合
docker compose exec rag-review rm -rf /app/chroma_db
docker compose restart rag-review
```

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
| `rag/constraints.txt` | 固定バージョン依存関係（**本番環境ではこちらを使用**） |
| `rag/requirements.txt` | 参考用（バージョン指定なし） |
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
pip install -r constraints.txt
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
