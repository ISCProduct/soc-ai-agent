# 企業 Search Provider コスト比較（#590）

企業情報 Write の Web 検索を `CompanySearchProvider` 経由に抽象化し、単価の高い OpenAI Search から Brave 等へ切り替えられるようにした。

## 切替

```bash
# 既定
COMPANY_SEARCH_PROVIDER=openai

# Brave Search API
COMPANY_SEARCH_PROVIDER=brave
BRAVE_SEARCH_API_KEY=BSA...
# 任意
# BRAVE_SEARCH_BASE_URL=https://api.search.brave.com/res/v1/web/search
```

`brave` 指定でも `BRAVE_SEARCH_API_KEY` が空なら **OpenAI にフォールバック**する。

実装: `Backend/internal/companyfetch/provider*.go`。呼び出しは `LLM.SearchLiteJSON` → Provider.Search。

## 単価感（目安・2026-07 設計値）

| Provider | 1 クエリ目安 | 備考 |
|----------|-------------|------|
| OpenAI Search Lite（`gpt-4o-mini-search-preview`） | **≈ $0.025** | ツール料金が支配的。実測は企業基本情報 Search→Parse で約この水準 |
| Brave Search API（Web Search） | **≈ $0.005 未満/クエリ帯**（プラン次第） | スニペット取得のみ。構造化は既存 Parse（mini）を別途課金 |
| OpenAI deep（`gpt-4o-search-preview`） | Search Lite より高い | 企業パイプラインでは使わない |

月 2,000 回の Write Search を仮定:

| 構成 | 概算 |
|------|------|
| すべて OpenAI Search Lite | ≈ **$50** |
| Brave + mini Parse（Parse を $0.002/回相当と仮置き） | ≈ **$10〜20** 台（プラン・Parse 回数に依存） |

※ Brave の正確な単価は契約プランで変わる。導入前に [Brave Search API Pricing](https://brave.com/search/api/) で確認すること。

## 品質差

- OpenAI Search: モデルが検索結果を要約/JSON 寄りに返すことがある（現行プロンプト前提）
- Brave: タイトル・URL・description の連結テキスト。**必ず Parse 段**で JSON 化する現行 `SearchLiteThenParse` と相性が良い

## 関連

- Issue #590 / 設計 `docs/design/company-info-acquisition.md` §9・R1
- 月次件数ガードは #587
