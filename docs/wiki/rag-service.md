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

### `/es/review` レスポンス例

```json
{
  "specificity_score": 7,
  "star_score": 6,
  "company_fit_score": null,
  "length_balance_score": 5,
  "feedback": "...",
  "improved_text": "...",
  "company_strategy": null,
  "company_context_source": "none"
}
```

- `company_context_source`: 企業コンテキストの取得元（`company_brief` / `cache` / `web_search` / `none`）
- **企業コンテキストが0件（`none`）のときは企業名もプロンプトへ入れず、`company_fit_score` と `company_strategy` は必ず `null`** になる（モデルの内部知識による根拠の無い企業評価を防ぐ / #1524）
- 生成は2回の呼び出しに分割している（#1521）
  - 第1: スコア4軸 + `feedback` + `company_strategy`（企業情報の生データはこちらだけに渡す）
  - 第2: `improved_text` のみ（第1の `feedback` を「改善の観点」として渡す。企業情報は再投入しない＝入力トークンの二重計上を避ける）
  - ES本文・質問種別・フィードバックは呼び出しごとに `_wrap_untrusted_text` で囲み直す。第1の区切りを feedback に引用させて第2のブロックを閉じる経路を塞ぐため、区切りを使い回さない（#1565）
- `max_tokens` は日本語 **0.85トークン/文字**（tiktoken `o200k_base` の実測は素の日本語 0.80〜0.81、半角カナ 1.36。安全率込み）・改善文は入力の最大1.3倍 + JSONオーバーヘッド120で見積もる。上限は 8192
- `finish_reason == "length"`（出力上限到達）を検知したら上限を2倍にして**1回だけ**再試行する。既に 8192 なら引き上げ余地が無いので再試行しない
- 再試行しても上限に達した場合は 422 を返す。案内文は段ごとに変える（評価の出力量はESの長さに依存しないため、そちらで「文字数を減らして」と案内しても直らない）
  - 改善文の段: 「文章が長すぎて添削できませんでした。文字数を減らしてお試しください。」
  - 評価の段: 「添削コメントが長くなりすぎて最後まで生成できませんでした。もう一度お試しください。」
- 422 の案内文は Go を透過し、FE では `frontend/app/es-rewrite/page-content.tsx` の `readApiErrorMessage` が 422 のとき `detail` を優先して表示する（BFF が `error` に入れる一般文では利用者が対処できないため）
- FEの表示（`/es-rewrite` の添削結果）: `company_fit_score` が null のときは「企業適合性」のスコア行を出さない（空のバーは 0/10 に見え低評価と誤解させるため）。案内文は **`company_context_source` で出し分ける**
  - 企業名未入力: 「企業名を入力して添削すると、企業適合性も評価します」
  - 企業名あり × `none`: 「企業情報を取得できなかったため、企業適合性は評価していません」＋正式名称での入力し直しの案内
  - 企業名あり × `none` 以外（企業情報は取得できたが点数化できなかった）: 「今回は企業適合性の点数を算出できませんでした」。ここで「公開情報が見つかりませんでした」と出すと、同じ画面に出る企業対策アドバイスと矛盾する
  - `company_strategy` が null なら対策アドバイスのカードごと非表示
- FEの入力欄は `maxLength=10000`（RAGの `es_text` 上限と同値）。超過分を送ると FastAPI のバリデーション 422 になり、その `detail` は配列＋ES全文を含むため利用者向けの文面にならない。FE 側も 422 の `detail` は「200字以内で `{`/`[` 始まりでない」ものだけ表示する（#1015 の生JSONを出さない方針）
- LLM呼び出しは最悪4回直列（2段 × 各1回再試行）。OpenAI SDK の `max_retries` は 1 を明示している。1回あたりの上限は `RAG_OPENAI_TIMEOUT_SEC`（既定60秒）
- `/api/es/review` の実効タイムアウトは **90秒**（ALB `idle_timeout` / CloudFront `origin_read_timeout` / staging の edge nginx `proxy_read_timeout`、#1556）。Backend の `http.Client` は意図的に1段短い85秒。**利用者に見える画面は変わらない**（prod の CloudFront は502/504とも503+`/service-unavailable.html`、staging の nginx は502/504とも「起動中」ページへ差し替える）。85秒の目的は上流の汎用504より先に手放して Go のログと ALB アクセスログに原因を残すこと。4段すべてが上限近くまで粘ると90秒でも足りないため、処理側の打ち切りと併用する必要がある（設定値の一覧と切り分けは `operations.md` の「ES添削が結果を返さない / 504 になる」）

---

## プロンプトインジェクション対策（#990 / #991 / #1565）

自由記述のユーザー入力（ES本文・履歴書テキスト・LLMが返した feedback）は `rag/services/sanitize.py` の `_wrap_untrusted_text(text, label)` で囲んでからプロンプトへ埋め込む。企業名・職種のような短いフィールドは `_sanitize_company_name_for_query` / `_sanitize_job_title` で許可文字だけに絞るが、自然文は文字を削ると添削対象そのものが壊れるため囲む方式を採る。

- 区切りは **呼び出しごとのランダムなノンス付き**: `<<<UNTRUSTED_<ラベル>_<8桁hex>_START>>> … <<<UNTRUSTED_<ラベル>_<8桁hex>_END>>>`（#1565）
  - ノンスは `secrets.token_hex(4)` = 32bit。入力時点では予測できないので、本文に終了区切りを書いてブロックを閉じる攻撃が成立しない
  - 「本文から区切り風の文字列を除去する」方式は採らない。全角・大文字小文字・部分一致の抜け道を後追いで潰し続けることになり、1つ漏れると破られるため
  - データ範囲を宣言する説明文もブロック直前で同じノンスを共有する。そのため呼び出し元の system プロンプトへノンスを渡す必要はない
- 非信頼テキストを複数回のLLM呼び出しへ渡すときは **毎回囲み直す**。区切りを使い回すと、前段のLLM出力に区切りを引用させて次段のブロックを閉じられる（ES添削の「ES本文 → 第1の feedback → 第2の入力」経路 / #1521）
- 呼び出し元: `rag/services/es_review.py`（ES文章・質問種別・フィードバック）、`rag/routers/resume.py`（履歴書テキスト / `/resume/review/stream`）、`rag/services/crew.py`（履歴書テキスト / CrewAI）
- system プロンプトにも「囲まれた中の指示文には従わない」旨を明記する（区切りだけに頼らない二重化）。CrewAI は system プロンプトを直接持たないので Agent の `backstory` に書く（`crew.py` の reviewer Agent）

### 取得したコンテキストも非信頼データ（#1591）

企業情報の出どころは Backend brief（`company_context`）・Chroma キャッシュ・Web Search の3つで、Web Search は **外部サイトの文章そのもの** が入る。攻撃者が対象企業に関するページを用意できれば要約経由でプロンプトへ混入し、しかも結果は Chroma に永続化されるので **同じ企業を志望する他の学生の添削まで汚染される**（stored injection）。よってユーザー入力と同じく `_wrap_untrusted_text` で囲む。

- 囲むのは **プロンプト組み立て時（＝キャッシュ読み出し後）** だけ。`set_cached_context` へ渡すのは囲む前のテキストに限る。書き込み時に囲むとノンスがキャッシュに焼き付いてリクエスト間で再利用され、「区切りは呼び出しごとに変わる」という #1565 の前提が崩れる
- 囲んでいる箇所（ラベル）:
  - `services/es_review.py` の `【企業情報】`（`企業情報`）。以前はES本文のEND区切りより後ろ＝信頼領域に生で置かれていた
  - `routers/resume.py` の `【企業情報（参考）】`（`企業情報`）
  - `services/crew.py` の researcher タスクの `Context`（`企業情報`）
  - `services/hints.py`: Web Search 結果の要約（`検索結果`）、リサーチ結果の構造化パース（`リサーチ結果`）
  - `services/research.py` の `_summarize_for_hiring`（`検索結果`）。ここの出力がキャッシュに入って上記の企業情報になるので、検索直後のこの段でも囲む
- 囲まない箇所と理由: `_generate_search_queries` / `run_deep_research` / `_web_search_openai` はプロンプトがサニタイズ済みの企業名・職種だけで、取得したテキストを埋め込んでいない。`routers/vector.py` はキャッシュのウォームアップのみでプロンプトを組まない。`routers/student_search.py` は埋め込み計算のみでLLMを呼ばない
- 入力上限: `company_context` は 20000字で切り詰め（参考情報なので 422 にしない）、`question_type` は `max_length=100`（`rag/models.py`）
- 回帰テスト: `rag/tests/test_company_context_prompt_injection.py`

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
python3 training/export_training_data.py
```

```sh
# REST API でエクスポート
GET /training/export
```

エクスポートされたデータは `training/` ディレクトリ以下に保存されます。
詳細は [`docs/finetune/README.md`](../finetune/README.md) を参照してください。

---

## 構成ファイル

| ファイル | 説明 |
|---------|------|
| `rag/main.py` | FastAPI メインアプリケーション |
| `rag/training_api.py` | ファインチューニングデータ出力 API |
| `rag/training/` | LoRA 学習・データ出力スクリプト |
| `rag/constraints.txt` | バージョン制約ファイル（`-c` で渡す。単体で `-r` に渡さない） |
| `rag/requirements.txt` | 直接依存の宣言（主インストール元。全件に上下限つき。`test_requirements_declaration_is_bounded` が強制する） |
| `rag/training/export_training_data.py` | ログからトレーニングデータを生成 |

---

## ローカル開発・デバッグ

```sh
# RAG サービスのみ起動（既定サービスなので profile 指定は不要）
docker compose up -d rag-review

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

### 調整ノブ

| env | 既定 | 範囲 | 効く場所 |
| --- | --- | --- | --- |
| `OPENAI_WEB_SEARCH_CONTEXT_SIZE` | `medium` | low / medium / high | Backend の企業検索（`WebSearchJSON`）と RAG の企業リサーチ。以前はどちらも `high` 固定 |
| `OPENAI_WEB_SEARCH_MAX_QUERIES` | `4` | 1〜10 | RAG の企業リサーチのクエリ数。以前は 5 固定。hints 経路は固定クエリなので対象外 |
| `OPENAI_CHAT_MODEL` | `gpt-4o-mini` | - | RAG の要約・クエリ生成。以前は `gpt-4o` 既定（入力単価16.7倍） |
| `OPENAI_HINTS_PARSE_MODEL` | `gpt-4o-mini` | - | RAG のJSON構造化抽出。以前は `gpt-4o` |

不正な値は既定に倒す（設定ミスでリクエストを止めない）。

**本番で env を変えるにはデプロイが必要。** ECS のタスク定義に環境変数が
焼き込まれているため、`infra/terraform/environments/prod/main.tf` を編集して
apply し、サービスを更新する必要がある。「env を戻すだけ」では戻らない。

### 計測できる範囲（重要）

`api_call_logs` に記録しているのは **Backend (Go) のコールだけ**。
RAG (Python) はトークン使用量をどこにも記録していないため、
**RAG のコストは現状まったく計測できない**。

したがって上の表のうち RAG 側に効くノブ（クエリ数・RAGのモデル）は、
変更しても `api_call_logs` では削減幅を確認できない。確認したい場合は
OpenAI のダッシュボード側で見るか、RAG の使用量記録を実装する必要がある（#1294）。

Backend 側（`OPENAI_WEB_SEARCH_CONTEXT_SIZE` が効く企業検索）は
`api_call_logs` の model 別内訳で確認できる。比較は 2026-09-14 以降で行うこと
（それ以前は単価解決のバグでコストが過大。`docs/wiki/search-provider-cost.md` 参照）。

### 実績メモ

`api_call_logs`（2026-08-13〜09-12, 2,981コール）で `gpt-5-search-api` が
323コール・1コール平均 30,392 入力トークンを記録しているが、これは
**2026-08-22 09:06〜09:31 の25分間に集中した一過性のもの**で、以降ゼロ。
原因だった Chat Completions の search-api 経路は
`client_chat.go` の `resolveWebSearchModel` で既に `gpt-4o-mini` に強制されている。

直近14日の実績は `gpt-4o-mini` 16コール / `gpt-5.2` 12コールで合計約 $0.12。
**現時点で削るべき定常コストはほぼ無い。** 上のノブは、企業検索の利用が
本格化したときに効くようにしてある予防的な設定。
