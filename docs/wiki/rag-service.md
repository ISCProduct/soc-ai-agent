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

### `/es/review` リクエスト例

```json
{
  "es_text": "学生時代に力を入れたことは...",
  "question_type": "学チカ",
  "company_name": "株式会社Example",
  "tech_stack": "Go, React",
  "char_limit": 400,
  "char_limit_mode": "within"
}
```

- `es_text` の上限は **6,000字**（#1564。後述「入力上限の決め方」）
- `tech_stack`（任意）はESリライト経路の入力。改善文の生成側にだけ渡す（#1533）
- `char_limit`（任意・100〜2000）は設問の文字数上限、`char_limit_mode` は `within`（「400字以内」＝超過不可・既定）/ `around`（「400字程度」＝+10%まで）（#1523）

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
  "company_context_source": "none",
  "star": { "situation": "...", "task": "...", "action": "...", "result": "..." },
  "improved_text_length": 392,
  "char_limit_satisfied": true,
  "usage": { "model": "gpt-4o", "prompt_tokens": 1200, "completion_tokens": 900, "calls": 2 }
}
```

- `company_context_source`: 企業コンテキストの取得元（`company_brief` / `cache` / `web_search` / `none`）
- **企業コンテキストが0件（`none`）のときは企業名もプロンプトへ入れず、`company_fit_score` と `company_strategy` は必ず `null`** になる（モデルの内部知識による根拠の無い企業評価を防ぐ / #1524）
- 生成は2回の呼び出しに分割している（#1521）
  - 第1: スコア4軸 + `feedback` + `company_strategy`（企業情報の生データはこちらだけに渡す）
  - 第2: `improved_text` + `star`（第1の `feedback` を「改善の観点」として渡す。企業情報は再投入しない＝入力トークンの二重計上を避ける）
- `max_tokens` は日本語 **0.85トークン/文字**（tiktoken `o200k_base` の実測は漢字かな混在 0.80〜0.81・ひらがな主体 約0.95・**半角カナ 約1.71**。0.85 は「素の日本語＋安全率」で、半角カナは覆っていない → 後述「入力上限の決め方」）・改善文は入力の最大1.3倍 + JSONオーバーヘッド120で見積もる。上限は 8192
- `finish_reason == "length"`（出力上限到達）を検知したら上限を2倍にして**1回だけ**再試行する。既に 8192 なら引き上げ余地が無いので再試行しない
- 再試行しても上限に達した場合は 422 を返す。案内文は段ごとに変える（評価の出力量はESの長さに依存しないため、そちらで「文字数を減らして」と案内しても直らない）
  - 改善文の段（`char_limit` なし）: 「文章が長すぎて添削できませんでした。文字数を減らしてお試しください。」
  - 改善文の段（`char_limit` あり）: 「指定字数に収められませんでした。文字数上限を緩めるか、もう一度お試しください。」
    `char_limit` があると出力予算は指定字数だけで決まり、ES本文の長さに依存しない。ここで「文字数を減らして」と案内すると、ES本文をいくら削っても直らない案内になる（#1523）
  - 評価の段: 「添削コメントが長くなりすぎて最後まで生成できませんでした。もう一度お試しください。」
- 422 の案内文は Go を透過し、FE では `frontend/app/es-rewrite/page-content.tsx` の `readApiErrorMessage` が 422 のとき `detail` を優先して表示する（BFF が `error` に入れる一般文では利用者が対処できないため）
- FEの表示（`/es-rewrite` の添削結果）: `company_fit_score` が null のときは「企業適合性」のスコア行を出さない（空のバーは 0/10 に見え低評価と誤解させるため）。案内文は **`company_context_source` で出し分ける**
  - 企業名未入力: 「企業名を入力して添削すると、企業適合性も評価します」
  - 企業名あり × `none`: 「企業情報を取得できなかったため、企業適合性は評価していません」＋正式名称での入力し直しの案内
  - 企業名あり × `none` 以外（企業情報は取得できたが点数化できなかった）: 「今回は企業適合性の点数を算出できませんでした」。ここで「公開情報が見つかりませんでした」と出すと、同じ画面に出る企業対策アドバイスと矛盾する
  - `company_strategy` が null なら対策アドバイスのカードごと非表示
- FEの入力欄は `maxLength=6000`（RAGの `es_text` 上限と同値 / #1564）。超過分を送ると FastAPI のバリデーション 422 になり、その `detail` は配列＋ES全文を含むため利用者向けの文面にならない。FE 側も 422 の `detail` は「200字以内で `{`/`[` 始まりでない」ものだけ表示する（#1015 の生JSONを出さない方針）
- LLM呼び出しは最悪8回直列（評価2回 ＋ 改善文3回 × 各1回再試行）。OpenAI SDK の `max_retries` は 1 を明示している。Backend 側の `/api/es/review` は 180秒だが、ALB(`idle_timeout` 既定60秒) / CloudFront(`origin_read_timeout` 60秒) が先に切るため実効は60秒（#1556 で対応）

### 設問の文字数上限（#1523）

- 字数の数え方は `services/es_review.py` の **`count_es_chars` が唯一の定義**: 改行と前後の空白は数えず、それ以外（全角・半角・記号・文中の空白）は1文字。プロンプトの指示・生成後の検査・`length_balance_score` の採点基準・FEの表示はすべてこの値に揃える（FEで数え直さない）
- 目標レンジは `_CHAR_LIMIT_RANGE`: `within` = 指定字数の85〜100%、`around` = 90〜110%
- 生成後にサーバ側で字数を検査し、許容上限を超えていたら**改善文だけ**を最大2回（`_MAX_CHAR_LIMIT_RETRIES`）作り直す。再生成プロンプトには「直前は N 字で、目標の M 字を超えた」と実測値を入れる（モデルの自己申告には頼らない）。評価（第1呼び出し）は作り直さない
- 再生成は**開始から45秒（`_CHAR_LIMIT_RETRY_DEADLINE_SEC`）を超えたら打ち切る**。手前のALB / CloudFront が60秒で切るため(#1556)、跨ぐと 422 の案内文も生成済みの本文も利用者へ届かない。打ち切りは失敗ではなく `char_limit_satisfied: false` として返す
- それでも収まらない場合は**切り詰めず**、`char_limit_satisfied: false` と `improved_text_length` を返す。FEは字数を出し、収まらなかったことを警告で明示する
- `char_limit` 指定時の改善文の出力予算は、入力長ではなく指定字数から見積もる（長いESを短く直す指定が通常ケース）

### ES添削とESリライトの統合（#1533）

- ESの評価・書き換えの実装は `services/es_review.py` のみ。Backend の `/api/es/review`（ES添削タブ）と `/api/es/rewrite`（ESリライトタブ）はどちらも RAG の `/es/review` を呼ぶ
- 旧実装では `/api/es/rewrite` が Backend 内の独自プロンプト（gpt-4o-mini・**インジェクション対策なし**・字数指示120〜150%）で生成していたため、同じESでも添削タブと違う書き換え案が返っていた。統合でプロンプト・出力スキーマ・インジェクション対策が1箇所になった
- `/api/es/rewrite` のレスポンスは従来互換（`rewritten_text` / `star`）。`improved_text` を `rewritten_text` に詰め替えて返し、`improved_text_length` / `char_limit_satisfied` を追加している
- STAR分解（`star`）は改善文と同じ第2呼び出しで生成する。ES添削タブは `star_score`、ESリライトタブは `star` の内訳を同じレスポンスから表示する
- **デプロイ順は rag-review を backend より先**にする（`.github/workflows/deployment.yml`）。Backend の ES 経路は生成を RAG へ委譲しており、旧 RAG は Pydantic の `extra=ignore` で `char_limit` / `tech_stack` を**黙って捨てる**ため、逆順だとロール中の数分だけ「字数上限を指定しても無視され、画面に手がかりも出ない」窓ができる
- コストの機能別内訳（`es_review` / `es_rewrite`）は、RAG が返す `usage` を Backend が `openai.Client.ReportProxyUsage` で `api_call_logs` へ記録して残す。RAG 自身は記録先を持たないため、この経路が唯一の記録手段（統合前は RAG 経由の添削ぶんが記録されていなかった）。`usage` は内部情報なので Backend が転送前に本文から取り除く

### 入力上限の決め方（#1564）

- `es_text` の上限 6,000字は「改善文（入力の最大130%）＋STAR分解(400字)を `_MAX_OUTPUT_TOKENS` 内で生成できる長さ」から決めている: `(6000 * 1.3 + 400) * 0.85 + 120 = 7,090` トークン < 8,192
- **この 6,000 は「素の日本語（漢字かな混在）での見積もり」に基づく値**。tiktoken(o200k_base) の実測レートは素材で大きく散り、漢字かな混在 0.80〜0.81 / ひらがな主体 約0.95 / **半角カナ 約1.71** tok/char。`_JP_TOKENS_PER_CHAR = 0.85` は全素材を覆っていない
- **未解決（#1564 は開けたまま）**: 半角カナ主体のESは約3,375字で天井に達するため、3,376〜6,000字では再試行しても足りず 422 になる。係数を素材別にするか `_MAX_OUTPUT_TOKENS` を上げるかは同Issueで継続検討する。テスト `test_halfwidth_kana_exceeds_the_cap_known_limitation` にこの限界を数値で残している
- `_MAX_OUTPUT_TOKENS` を引き上げる案は採らなかった。`OPENAI_CHAT_MODEL` は環境変数で差し替えられ、出力上限4,096のモデルでは API 400 になるリスクが増すうえ、1回の生成コストも上がる。実務のESは400〜800字が中心で、`char_limit` が入ると長大なESを投げる動機自体が減る
- 上限を超えた入力は FastAPI のバリデーションで弾く（FEの `maxLength=6000` で手前でも止める）。**以前は 7,300〜10,000字が「送れるが必ず422」の帯**で、しかも2回の生成に課金してから失敗していた
- テストで固定している不変条件: `tests/test_models.py::test_max_length_output_fits_in_output_cap_for_plain_japanese`（入力上限いっぱいの**丸める前の**見積もりが出力天井未満であること。`_estimate_max_tokens` の戻り値は天井で丸められるため、そのまま比較しても「飽和したか」しか分からない）

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
