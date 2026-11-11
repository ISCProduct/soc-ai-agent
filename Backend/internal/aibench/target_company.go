package aibench

import (
	"context"
	"strings"
	"unicode"

	"Backend/internal/companyfetch"
	"Backend/internal/services/company"
)

// companyExtractTarget は事実テキストからの企業情報の抽出（company_info_fetcher.go の
// acquireCheapExtract）を評価する（#1782）。
//
// 本番と同じプロンプト・スキーマ（Structured Outputs）・出力上限・温度で呼び、本番と同じ
// 手順（ParseCompanyInfoResult: JSON の取り出し・業種の正規化・範囲チェック）で読む。
// 点は「正解の項目と一致した割合」。事実テキストに無い項目へ値を入れた場合は、
// 根拠の無い値を作った（fabricated）として外れに数える。
type companyExtractTarget struct {
	model string
}

// companyExtractTemperature は本番（companyfetch.LLM.ExtractJSONWithSchema）の温度。
const companyExtractTemperature = 0.2

// companyExtractPromptOverheadTokens はシステムプロンプトとユーザープロンプトの定型部分の概算。
const companyExtractPromptOverheadTokens = 900

// companyExtractFields は absent に書ける項目（スキーマのキー名）。
var companyExtractFields = []string{
	"description", "industry", "location", "website_url", "founded_year", "employee_count",
	"main_business", "culture", "work_style", "tech_stack", "welfare_details",
}

func newCompanyExtractTarget(modelOverride string) Target {
	m := strings.TrimSpace(modelOverride)
	if m == "" {
		// 本番と同じ既定（OPENAI_COMPANY_EXTRACT_MODEL があればそれ）
		m = companyfetch.ExtractModel()
	}
	return &companyExtractTarget{model: m}
}

func (t *companyExtractTarget) Name() string     { return TargetCompanyExtract }
func (t *companyExtractTarget) Model() string    { return t.model }
func (t *companyExtractTarget) Endpoint() string { return "openai:/chat/completions(json_schema)" }

func (t *companyExtractTarget) EstimateTokens(c Case) (int, int) {
	return ApproxTokensJA(c.Input.FactsText) + ApproxTokensJA(c.Input.CompanyName) + companyExtractPromptOverheadTokens,
		company.CompanyInfoExtractMaxTokens
}

func (t *companyExtractTarget) Run(ctx context.Context, c Case) Observation {
	system, user := company.CompanyInfoExtractPrompts(c.Input.CompanyName, c.Input.WebsiteURL, c.Input.FactsText)
	schema := company.CompanyInfoResponseSchema()
	var res CallResult
	if schema == nil {
		res = CallChatCompletions(ctx, t.model, system, user, companyExtractTemperature, company.CompanyInfoExtractMaxTokens, true)
	} else {
		res = CallChatCompletionsWithSchema(ctx, t.model, system, user, companyExtractTemperature,
			company.CompanyInfoExtractMaxTokens, schema.Name, schema.Schema)
	}
	return evaluateCompanyExtractResponse(newObservation(c), *c.Input.Expected, res)
}

// evaluateCompanyExtractResponse は LLM の生出力を指標へ落とす（純粋関数）。
func evaluateCompanyExtractResponse(o Observation, expected CompanyExtractExpected, res CallResult) Observation {
	o.LatencyMS = res.LatencyMS
	o.PromptTokens = res.PromptTokens
	o.CompletionTokens = res.CompletionTokens
	if res.Truncated {
		o.Broken, o.BrokenReason, o.Error = true, BrokenTruncated, "max_tokens に到達"
		return o
	}
	if res.Err != nil {
		reason := BrokenCallFailed
		if res.Unmeasured {
			reason = BrokenNetwork
		}
		o.Broken, o.BrokenReason, o.Error, o.Fatal = true, reason, res.Err.Error(), res.Fatal
		return o
	}
	result, err := company.ParseCompanyInfoResult(res.Text)
	if err != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenJSONInvalid, err.Error()
		return o
	}
	passed, total, violations := scoreCompanyExtract(*result, expected)
	o.Violations = append(o.Violations, violations...)
	o.RawScore = float64(passed)
	o.Score = float64(passed) / float64(total)
	return o
}

// companyExtractCheckCount は正解で確かめる項目の数。
func companyExtractCheckCount(e CompanyExtractExpected) int {
	n := len(e.Absent)
	for _, set := range []bool{e.Industry != "", e.Location != "", e.MainBusiness != "", e.TechStack != "", e.FoundedYear != 0, e.EmployeeCount != 0,
		e.EmployeeCountBasis != "", e.WorkStyle != "", e.WelfareDetails != ""} {
		if set {
			n++
		}
	}
	return n
}

// scoreCompanyExtract は正解の項目ごとに当否を数える。
func scoreCompanyExtract(r company.CompanyInfoResult, e CompanyExtractExpected) (passed, total int, violations []string) {
	check := func(name string, ok bool) {
		total++
		if ok {
			passed++
		} else {
			violations = append(violations, "field_mismatch:"+name)
		}
	}
	if e.Industry != "" {
		check("industry", r.Industry == e.Industry)
	}
	if e.Location != "" {
		check("location", containsNormalized(r.Location, e.Location))
	}
	if e.MainBusiness != "" {
		check("main_business", containsNormalized(r.MainBusiness, e.MainBusiness))
	}
	if e.TechStack != "" {
		check("tech_stack", containsNormalized(r.TechStack, e.TechStack))
	}
	if e.FoundedYear != 0 {
		check("founded_year", r.FoundedYear == e.FoundedYear)
	}
	if e.EmployeeCount != 0 {
		check("employee_count", r.EmployeeCount == e.EmployeeCount)
	}
	if e.EmployeeCountBasis != "" {
		check("employee_count_basis", r.EmployeeCountBasis == e.EmployeeCountBasis)
	}
	if e.WorkStyle != "" {
		check("work_style", r.WorkStyle == e.WorkStyle)
	}
	if e.WelfareDetails != "" {
		check("welfare_details", containsNormalized(r.WelfareDetails, e.WelfareDetails))
	}
	for _, field := range e.Absent {
		total++
		if companyFieldEmpty(r, field) {
			passed++
		} else {
			violations = append(violations, "fabricated:"+field)
		}
	}
	return passed, total, violations
}

func companyFieldEmpty(r company.CompanyInfoResult, field string) bool {
	switch field {
	case "description":
		return strings.TrimSpace(r.Description) == ""
	case "industry":
		// 業種はスキーマが選択肢を強制するので「その他」は空とみなす
		return r.Industry == "" || r.Industry == "その他"
	case "location":
		return strings.TrimSpace(r.Location) == ""
	case "website_url":
		return strings.TrimSpace(r.WebsiteURL) == ""
	case "founded_year":
		return r.FoundedYear == 0
	case "employee_count":
		return r.EmployeeCount == 0
	case "main_business":
		return strings.TrimSpace(r.MainBusiness) == ""
	case "culture":
		return strings.TrimSpace(r.Culture) == ""
	case "work_style":
		return strings.TrimSpace(r.WorkStyle) == ""
	case "tech_stack":
		return strings.TrimSpace(r.TechStack) == ""
	case "welfare_details":
		return strings.TrimSpace(r.WelfareDetails) == ""
	}
	return false
}

// containsNormalized は空白と大文字小文字を無視して部分一致を見る。
func containsNormalized(text, want string) bool {
	norm := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || r == '　' {
				return -1
			}
			return unicode.ToLower(r)
		}, s)
	}
	return strings.Contains(norm(text), norm(want))
}
