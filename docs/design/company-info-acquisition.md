# 企業情報取得方式の設計（#557）

> 目的: LLM を「最新テキストの抽出器」として使い続けつつ、1企業あたりのトークンを現状比 50% 以上削減し、フィールド単位で鮮度を担保する。

関連: Issue [#557](https://github.com/ISCProduct/soc-ai-agent/issues/557) / Backlog `SOCAIAGENT-29`  
参考実装: `CompanyValidationService`（DB → 軽量 WebSearch）、RAG hints（Search → Parse）

---

## 1. 調査サマリ（結論）

| 問い | 結論 |
|------|------|
| トークン 50% 削減は可能か | **可能**。通常ルートを「スクレイプ/gBiz + mini 抽出」に寄せ、Search をフォールバックに限定すれば 2,000〜3,000 → **〜1,200 tokens** が現実的 |
| 最新性が必要なフィールドは | **求人（TTL 7日）・技術スタック（30日）・企業関係（60日）** が最優先。法人番号・所在地は gBiz で足りる |
| モデル使い分け | Extract=`gpt-4o-mini`、Search-Lite=`gpt-4o-mini-search-preview`、Deep=`gpt-4o-search-preview`、Parse=`gpt-4o-mini`（複雑時のみ `gpt-4o`） |
| 既存資産との統合 | gBiz / company-graph スクレイプ / HINTS 2段 / Validation の **階段フォールバック** を 1 パイプラインに統合する |

**現状の最大問題:** エンドポイント名が `web-search` / `fetch-info` でも、実装の大半は `ChatCompletionJSON`（**モデル知識**）であり、学習カットオフ以降の求人・Tech Stack・関係は保証できない。

---

## 2. 現行パス棚卸し

### 2.1 取得経路一覧

| 用途 | 実装 | 実体 | モデル | 鮮度 | FE / 呼び出し |
|------|------|------|--------|------|---------------|
| 企業基本情報 | `CompanyInfoFetcher.FetchAndSave` | モデル知識 | `OPENAI_MODEL` / mini, max 600 | ❌ | `POST .../fetch-info`、バッチ `fetch_info_all` |
| 企業基本（プレビュー） | `WebSearchCompanyInfo` | モデル知識（名前が WebSearch） | 同上 | ❌ | admin `companies/[id]/info` |
| 求人 | `CareersScraper.FetchJobs` → `JobFetchService` | モデル知識（`Source: ai_knowledge`） | mini, max 1500 | ❌ | `.../fetch-jobs` |
| 人物像 | `JobFetchService.FetchAndSavePersona` | DB テキスト → LLM | mini, max 500 | △（DB依存） | `.../fetch-persona` |
| 技術スタック | `FetchTechStack` | モデル知識 | mini, max 400 | ❌ | `.../tech-stack-search` |
| 企業関係 | `EnrichRelations` / `fetchRelationsWithLLM` | モデル知識（`SourceType: llm_web_search`） | mini, max 800 | ❌ | `POST .../enrich-relations` |
| 公的データ | `GBizInfoService` | gBizINFO API | なし | ✅（API依存） | gBiz sync / search |
| 就活サイト | `tools/company-graph` + `Backend/internal/scraper` | HTML スクレイプ | なし（抽出時は mini） | ✅〜△ | company-graph crawl |
| クロール抽出 | `CrawlService` + OpenWork | HTML → LLM 抽出 | mini, HTML 最大約 12k 字 | ✅（ページ鮮度） | crawl sources |
| 企業実在確認 | `CompanyValidationService` | **DB → WebSearchJSON**（max 200） | `OPENAI_WEB_SEARCH_MODEL` | ✅ | `/api/companies/validate`（履歴書） |
| 候補検索 | `WebSearchCompanies` | WebSearch、失敗時はモデル知識 | search-preview / mini | △ | `/api/companies/web-search` |
| 面接ヒント | RAG `_run_hints_web_search_pipeline` | **真の Web Search ×3 → 要約 → Parse** | search-preview + chat + parse | ✅ | `/company/hints` |
| 面接 realtime プロフィール | `lookupCompanyProfile` | モデル知識（Responses） | デフォルトモデル | ❌ | Interview BE のみ |
| 履歴書/ES 文脈 | RAG `_run_web_search_pipeline` / Deep Research | 真の Search | search-preview 等 | ✅ | resume/ES → RAG |

### 2.2 OpenAI クライアント能力

`Backend/internal/openai/client.go`:

| API | 用途 | 既定モデル env |
|-----|------|----------------|
| `ChatCompletionJSON` | JSON 抽出・モデル知識 | `OPENAI_MODEL`（`gpt-4o-mini`） |
| `WebSearchJSON` | 短文 Web 検索 JSON | `OPENAI_WEB_SEARCH_MODEL`（`gpt-4o-search-preview`） |
| `Responses*` | realtime / プロフィール等 | `OPENAI_MODEL` 系 |

### 2.3 DB の鮮度メタデータ（現状）

`companies`（`Backend/internal/models/company.go`）:

- あり: `SourceType`, `SourceURL`, `SourceFetchedAt`, `GBizLastSyncedAt`, `GBizSyncStatus`
- **なし:** `confidence`, `model_used`, フィールド単位の `*_fetched_at`

`company_relations` に source / fetched_at はなく、説明文に `llm_web_search:...` を埋め込む程度。  
Validation の `confidence` は **メモリキャッシュ（30分）のみ**で永続化されない。

### 2.4 env / 実装のずれ

| 変数 | 意図 | 実態 |
|------|------|------|
| `OPENAI_HINTS_MODEL` | RAG hints の Search | `rag/main.py` は **`OPENAI_WEB_SEARCH_MODEL`** を参照。HINTS_MODEL は未使用 |
| `OPENAI_HINTS_PARSE_MODEL` | hints JSON 化 | RAG で使用（既定 `gpt-4o`） |
| `OPENAI_WEB_SEARCH_MODEL` | Go `WebSearchJSON` + RAG Search | `.env.example` に **未記載** |
| Brave Search MCP | `mcp/README.md` | **`compose.mcp.yml` がリポジトリに存在しない**。アプリコードからの呼び出しもなし |

### 2.5 参考になる既存パターン

1. **`CompanyValidationService`（推奨の原型）**  
   キャッシュ → DB → 必要時のみ `WebSearchJSON`（max 200）。根拠 URL・confidence・source を返す。
2. **RAG hints 2段**  
   Search（複数クエリ）→ 要約 → Parse。鮮度は良いがクエリ数が多くトークンが重い。企業情報側では **1クエリ + mini parse** に圧縮する。
3. **CrawlService の「HTML → 抽出」**  
   モデル知識ではなく入力テキストからの抽出。#557 の本流に近い。

---

## 3. フィールド別 鮮度要件 × Primary ソース × モデル

| フィールド | 鮮度 | TTL | Primary | Fallback | LLM |
|-----------|------|-----|---------|----------|-----|
| 法人番号・正式名称 | 低 | 365日 | gBizINFO | — | 同名曖昧さ解消時のみ mini |
| 所在地 | 低 | 180日 | gBizINFO | 公式サイト抽出 | 未充足時 mini |
| 従業員数 | 中 | 90日 | gBiz / 有報テキスト | Search-Lite | mini extract |
| 企業概要・事業内容 | 中 | 90日 | 公式サイトスクレイプ | mini-search | mini extract |
| **求人** | **高** | **7日** | careers スクレイプ / 就活サイト | Search-Lite → Parse | **禁止: モデル知識のみ** |
| **技術スタック** | **高** | **30日** | 採用/engineering ページ | Search-Lite → Parse | 同上 |
| 文化・働き方 | 中 | 60日 | 採用ページ | Search-Lite | mini |
| 企業関係 | 中 | 60日 | gBiz 調達/補助金 + 既存グラフ | Search-Full → Parse | mini / 複雑時 4o |
| 平均年齢・女性比率 | 低 | 180日 | gBiz workplace / 有報 | — | 原則 LLM 不要 |

判定ルール: `field_fetched_at`（または既存 `SourceFetchedAt` / `GBizLastSyncedAt`）+ TTL。期限内は再取得しない。未充足フィールドのみパイプラインを走らせる。

---

## 4. Search 系モデルの使用条件

### 使ってよい条件

- Primary（gBiz / スクレイプ）が **失敗**、または **URL 不明**
- 高鮮度フィールドが **TTL 超過** かつスクレイプ不能
- UI の企業実在確認・候補補完（Validation と同系統、短文）

### 使ってはいけない条件

- 企業名だけ渡して「知っている求人/Tech/関係を全部出して」
- TTL 内のキャッシュがあるフィールドの再 Search
- 全フィールドを毎回 `gpt-4o-search-preview` で一括生成
- スクレイプ成功済みテキストがあるのに Search を重ねる（Extract のみで足りる）

### 2段構成（Search → Parse）を採用するタスク

| タスク | Search | Parse | 備考 |
|--------|--------|-------|------|
| 求人（スクレイプ失敗時） | mini-search-preview | mini | CareersScraper 移行の本命 |
| 技術スタック（同上） | mini-search-preview | mini | |
| 企業関係（gBiz 不足時） | search-preview | mini（複雑長文は 4o） | 根拠 URL 必須 |
| 企業名 UI 補完 | mini-search（短文） | 不要 or mini | Validation 拡張 |

面接 hints は既に 2 段。企業情報パイプラインとは **env を分離**し、hints 側はクエリ数削減を別 Issue で検討する。

---

## 5. 統合パイプライン（目標アーキテクチャ）

```
企業名 / company_id / URL
        │
        ▼
[1] gBizINFO 名前検索 → 法人番号確定          (tokens: 0)
        │ TTL 内なら Sync スキップ
        ▼
[2] gBizINFO Sync → 構造化フィールド更新      (tokens: 0)
        │
        ▼
[3] 公式 / 就活 / careers URL スクレイプ      (tokens: 0)
        │ 失敗・URL不明
        ├──────────────────────────────┐
        ▼                              ▼
[4] テキスト trim(500〜1500字)     [3b] Search-Lite
        │                              │
        ▼                              ▼
[5] EXTRACT (gpt-4o-mini)          PARSE (gpt-4o-mini)
        │                              │
        └──────────┬───────────────────┘
                   ▼
[6] 高鮮度フィールドが未充足 / TTL 超過?
        │ Yes → Search-Lite → (不足時のみ Deep Search) → Parse
        ▼
[7] DB 保存: source_type / source_url / *_fetched_at / confidence / model_used
```

### タスク × モデル ルーティング（確定案）

| タスク | Step1 | Step2 | モデル |
|--------|-------|-------|--------|
| 法人番号 | gBiz 名前検索 | 候補≤5 の曖昧さ解消 | mini（必要時） |
| 基本プロファイル | gBiz Sync | 未充足のみ公式スクレイプ→extract | mini |
| 概要 | 公式スクレイプ | extract | mini |
| 求人 | careers / 就活スクレイプ | 失敗時 Search-Lite→Parse | mini-search → mini |
| Tech Stack | 採用ページスクレイプ | 失敗時 Search-Lite→Parse | mini-search → mini |
| 文化 | 採用ページ | extract | mini |
| 関係 | gBiz + 既存グラフ | 不足時 Search-Full→Parse | search-preview → mini/4o |
| UI 実在確認 | DB | WebSearch 短文 | 既存 Validation |

---

## 6. トークン見積もりと目標上限

### 現状（1企業フル取得・モデル知識）

| 呼び出し | max_tokens 目安 | 備考 |
|----------|-----------------|------|
| FetchInfo | 600 | |
| FetchJobs | 1500 | |
| FetchTechStack | 400 | |
| EnrichRelations | 800 | |
| **合計出力上限** | **≈ 3,300** | 入力プロンプト込みで実測 **2,000〜3,000+**、鮮度 ❌ |

### 提案

| シナリオ | 構成 | 推定トークン | 鮮度 |
|----------|------|-------------|------|
| 通常（URL あり） | スクレイプ + mini extract ×2〜3 | **800〜1,200** | ✅ |
| Search フォールバック | Search-Lite×1〜2 + Parse×2 + Extract | **1,500〜2,500** | ✅ |
| 最悪（避ける） | search-preview ×4 | 8,000+ | ✅だが非スケール |

### 目標上限（完了条件）

| 区分 | 上限 |
|------|------|
| 通常ルート（1企業フル） | **1,200 tokens** |
| Search フォールバック込み | **2,500 tokens** |
| UI 実在確認（1クエリ） | **200 tokens**（現行維持） |

50% 削減: 現状中央値 ≈2,500 → 提案通常 1,200 で **約 52% 減**（かつ鮮度改善）。

---

## 7. env 変数設計

既存 `OPENAI_MODEL` / HINTS 系と分離する。

```bash
# 抽出（高頻度・低コスト）
OPENAI_COMPANY_EXTRACT_MODEL=gpt-4o-mini

# Web検索（スクレイプ失敗・URL不明時の第一選択）
OPENAI_COMPANY_SEARCH_MODEL=gpt-4o-mini-search-preview

# Web検索フォールバック（不足時のみ）
OPENAI_COMPANY_DEEP_SEARCH_MODEL=gpt-4o-search-preview

# Search結果の構造化（2段目）
OPENAI_COMPANY_PARSE_MODEL=gpt-4o-mini

# 長文・関係抽出の高精度 Parse（必要時のみ）
OPENAI_COMPANY_PARSE_MODEL_ADVANCED=gpt-4o
```

運用メモ:

- Go の既存 `OPENAI_WEB_SEARCH_MODEL` は Validation / 公開 web-search で継続利用可。企業パイプラインは上記に寄せ、段階的に読み替え。
- RAG の `OPENAI_HINTS_MODEL` は **未使用**のため、実装 Issue で `OPENAI_WEB_SEARCH_MODEL` への統一か、hints 専用読込のどちらにするか決める（本設計では企業パイプラインと分離を推奨）。

---

## 8. DB スキーマ案

### 8.1 最小追加（Phase 1）

`companies` に追加:

| カラム | 型 | 用途 |
|--------|-----|------|
| `info_fetched_at` | datetime NULL | 基本情報の最終取得 |
| `jobs_fetched_at` | datetime NULL | 求人 |
| `tech_fetched_at` | datetime NULL | 技術スタック |
| `relations_fetched_at` | datetime NULL | 関係（企業単位の最終 enrich） |
| `last_model_used` | varchar(64) NULL | 直近に使ったモデル名 |
| `last_fetch_confidence` | varchar(16) NULL | high/medium/low |

既存 `SourceType` / `SourceURL` / `SourceFetchedAt` は「企業レコード全体の出典」として維持。フィールド TTL は上記 `*_fetched_at` で判定。

### 8.2 関係の provenance（Phase 1.5）

`company_relations` に `source_type`, `source_url`, `fetched_at`, `confidence` を追加し、説明文埋め込みをやめる。

### 8.3 既存 `llm_web_search` / `ai_knowledge` データの扱い

| 方針 | 内容 |
|------|------|
| 即削除しない | マッチング・面接が壊れるのを避ける |
| 再取得トリガー | TTL 超過 or admin「強制再取得」or バッチ |
| マーキング | `SourceType` が `llm_web_search` / 求人 `ai_knowledge` のレコードは **鮮度信頼度 low** 扱いし、UI で警告 |
| 上書き規則 | Primary（gBiz/スクレイプ）成功時は必ず上書き。Search 結果は confidence medium 以上のみ上書き |

---

## 9. Brave Search MCP 評価

| 項目 | 評価 |
|------|------|
| 現状 | README のみ。`compose.mcp.yml` 欠落。アプリ未接続 |
| 利点 | OpenAI Search 以外の検索コスト最適化、根拠 URL の明示 |
| 欠点 | 運用面（API キー・MCP プロセス）、Go サービスからの直接呼び出し設計が未整備 |
| 本設計での位置づけ | **Phase 2 候補**。Phase 1 は OpenAI Search-Lite/Full で統一し、インターフェース（`CompanySearchProvider`）だけ抽象化して後から Brave を差し込めるようにする |

---

## 10. 公式サイトスクレイプ（方針・リスク）

| 項目 | 方針 |
|------|------|
| 対象 | 公式 `WebsiteURL`、`/careers` `/recruit` `/engineering` など既知パス |
| robots.txt | 取得前に確認。Disallow はスキップし Search にフォールバック |
| 利用規約 | 就活サイトは既存 company-graph セレクタを優先。規約違反リスクのあるサイトは追加しない |
| テキスト長 | 抽出前に 500〜1,500 字（セクション見出し優先）へ trim |
| 失敗時 | Search-Lite（URL 特定）→ 再スクレイプ or 直接 Parse |

PoC（実装前の確認項目）: 大手〜スタートアップ 10 社で (a) スクレイプ成功率 (b) mini 抽出のフィールド充足率 (c) Search フォールバック率 (d) トークン実測。

---

## 11. CareersScraper 移行設計

現状: `FetchJobs` がモデル知識で `Source: "ai_knowledge"`。

移行後:

```
1. website_url / 既知 careers URL をスクレイプ
2. 成功 → EXTRACT(mini) → JobPostingResult[]（source=scrape）
3. 失敗 → SEARCH(mini-search) で求人ページ URL/スニペット取得
4. PARSE(mini) → JobPostingResult[]（source=web_search, evidence_urls 必須）
5. jobs_fetched_at 更新。TTL 7日以内はスキップ
```

`JobFetchService` の入口は変えず、内部実装だけ差し替え可能にする。

---

## 12. 段階的移行計画

### Phase 1（現行実装・暫定）

**方針（コスト優先）:**
- **OpenAI Web Search（search-preview 系）は使わない**（コスト过高）
- **公式サイトスクレイプも本流にしない**（運用負荷）
- **本流: gBizINFO（トークン 0）** で法人情報・所在地・従業員数・URL 等を取得
- **不足フィールドのみ** `OPENAI_COMPANY_EXTRACT_MODEL`（既定 gpt-4o-mini）で事実テキストから構造化
- 求人・Tech Stack は暫定で Extract のみ（`source=llm_extract`, confidence=low）。鮮度が必要な場合は手動 or 将来の安価 Search（Brave 等）

**目的:** 高額な Search モデルを避けつつ、モデル知識丸投げより根拠のある取得へ寄せる。

1. 企業情報専用 env（Extract 中心）と TTL
2. `CompanyInfoFetcher` を gBiz 優先 + 安価 Extract 補完に変更（Web Search 廃止）
3. `CareersScraper` / `FetchTechStack` から Web Search を除去
4. admin に gBiz / provenance 表示
5. main で `GBizInfoService` を AdminCompanyController に配線

**Phase 1 対象外:** EnrichRelations 全面書き換え、Brave MCP、面接 realtime プロフィール、RAG hints クエリ削減、スクレイプ本流化。

### Phase 2

- `EnrichRelations` を Search→Parse + relation provenance カラム化
- `CompanySearchProvider` に Brave を接続（compose.mcp.yml 整備）
- フィールド単位取得 API（未充足のみ）
- バッチ再取得ジョブ（TTL 超過のみ）

### Phase 3

- 面接 `lookupCompanyProfile` を DB/キャッシュ由来に変更（モデル知識廃止）
- RAG 履歴書パイプラインと企業パイプラインのキャッシュ共有
- トークン・コストのダッシュボード（`api_call_log` 活用）

---

## 13. 未決事項・リスク

| ID | 内容 | 影響 | 提案 |
|----|------|------|------|
| R1 | スクレイプ成功率（特に JS レンダリング必須サイト） | Search フォールバック増 → コスト増 | 静的 HTML 優先。失敗率 >40% なら Search-Lite を標準寄りに |
| R2 | OpenAI Search モデルの API 仕様変更 | パイプライン停止 | Provider 抽象化 |
| R3 | gBiz レート制限・トークン未設定環境 | Primary 欠落 | 開発はモック、本番は必須化 |
| R4 | HINTS_MODEL と WEB_SEARCH_MODEL の二重定義 | 運用混乱 | Phase 1 でドキュメント統一、コードは追従 Issue |
| R5 | 既存 ai_knowledge 求人の一括再取得コスト | 一時的コスト増 | TTL + 手動/夜間バッチ |
| R6 | robots.txt / 利用規約 | 法務・運用 | 許可サイトの allowlist |
| U1 | PoC 10 社の実測（精度・トークン）は本 PR 時点では未実施 | 数値の確度 | Phase 1 着手前に別コミットで計測シート追加 |
| U2 | `search-gbiz` FE パスと BE ルートの不一致疑い | admin UX | 実装時にルート突合 |

---

## 14. Phase 1 実装スコープ（確定）

**含む**

- env 5 種の配線と `.env.example` 反映
- Info / Jobs / TechStack の「外部テキスト → Extract」化
- フィールド別 `*_fetched_at` + TTL スキップ
- source / confidence / model_used の保存と admin 表示
- CareersScraper の ai_knowledge 廃止（フォールバックは Search→Parse）

**含まない**

- 関係グラフの全面刷新
- Brave MCP 本番接続
- RAG / 面接 hints の大規模改修

**受け入れ指標**

- 通常 1 企業フル取得 ≤ 1,200 tokens（ログ計測）
- 求人・Tech の新規取得で `source` が `scrape` または `web_search`（`ai_knowledge` 新規禁止）
- TTL 内の再実行で LLM 呼び出し 0

---

## 15. 調査タスクチェックリスト（#557）

- [x] 現行エンドポイント・FE 連携の棚卸し
- [x] フィールド別鮮度要件（TTL）の確定案
- [x] タスク × モデル ルーティング表
- [x] CareersScraper の Search+Parse 移行設計
- [x] HINTS 2段の企業情報への転用方針（採用・env 分離）
- [x] 公式スクレイプ方針（robots / trim / フォールバック）
- [x] Brave Search MCP 評価（Phase 2、抽象化のみ Phase 1）
- [x] 新 env・`.env.example` 反映案
- [x] DB スキーマ案（`*_fetched_at` / model / confidence）
- [x] 既存 `llm_web_search` / `ai_knowledge` 再取得方針
- [ ] PoC 10 社の実測（精度・トークン）← **実装 Phase 1 着手前に実施**

---

## 16. 参考コードパス

- `Backend/internal/services/company_info_fetcher.go`
- `Backend/internal/services/company_validation_service.go`
- `Backend/internal/services/job_fetch_service.go`
- `Backend/internal/services/gbizinfo_service.go`
- `Backend/internal/services/crawl_service.go`
- `Backend/internal/scraper/careers_scraper.go`
- `Backend/internal/controllers/admin_company_controller.go`
- `Backend/internal/controllers/admin_company_graph_controller.go`
- `Backend/internal/openai/client.go`
- `Backend/internal/models/company.go`
- `rag/main.py`（hints / web_search）
- `tools/company-graph/internal/scraper/`
- `mcp/README.md`
- `docs/design/vector-db.md`
