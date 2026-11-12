package company

import appopenai "Backend/internal/openai"

// 事実テキストからの企業情報の抽出（acquireCheapExtract）で LLM に渡すもの。
// aibench（#1782）が本番と同じプロンプト・スキーマ・上限で測れるよう公開する。

// companyExtractFactsSystemPrompt は事実テキストからの抽出のシステムプロンプト。
const companyExtractFactsSystemPrompt = `あなたは企業情報の構造化アシスタントです。与えられた事実テキストに書かれている内容のみをJSON化してください。テキストに無い項目は空文字または0。推測禁止。`

// CompanyInfoExtractMaxTokens は抽出の出力上限。
const CompanyInfoExtractMaxTokens = 600

// CompanyInfoExtractPrompts は事実テキストからの抽出で LLM に渡すシステム・ユーザープロンプトを返す。
func CompanyInfoExtractPrompts(companyName, websiteURL, factsText string) (system, user string) {
	return companyExtractFactsSystemPrompt, buildCompanyInfoExtractUser(companyName, websiteURL, factsText)
}

// CompanyInfoResponseSchema は Structured Outputs へ渡すスキーマ（本番と同じ）。
func CompanyInfoResponseSchema() *appopenai.ResponseSchema { return companyInfoResponseSchema() }

// ParseCompanyInfoResult は LLM の出力を本番と同じ手順（JSON の取り出し・正規化）で読む。
func ParseCompanyInfoResult(text string) (*CompanyInfoResult, error) {
	return parseCompanyInfoResult(text)
}
