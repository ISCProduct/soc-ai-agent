package company

import (
	"Backend/internal/companyfetch"
	"Backend/internal/config"
	"Backend/internal/services/shared"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const companyRelationsJSONSchema = `{
  "subsidiaries": [{"name": "議決権50%以上の子会社名", "ratio": 出資比率パーセント（不明なら省略）, "description": "資本関係の具体内容（例: 完全子会社・議決権○%）"}],
  "affiliates": [{"name": "議決権50%未満の資本提携・関連会社名", "ratio": 出資比率パーセント（不明なら省略）, "description": "資本関係の具体内容（例: 資本業務提携・出資比率○%）"}],
  "business_partners": [{"name": "資本関係が無い取引先・業務提携先または省庁・自治体名（曖昧・JV・その他は禁止）", "description": "取引内容。公開事実が無くても業界・事業内容から妥当に推定できる内容を書いてよい。推定もできず種別しか分からない場合は空文字（表示は主要取引先）"}],
  "market_info": {
    "is_listed": true/false,
    "market_type": "prime|standard|growth|unlisted",
    "stock_code": "証券コード（4桁、非上場なら空文字）"
  }
}`

func (f *CompanyRelationsFetcher) acquireViaAISearch(ctx context.Context, companyName, websiteURL string) (*CompanyRelationsResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	systemPrompt := `あなたは日本企業の資本関係・取引関係・上場情報に詳しいアシスタントです。検索結果と、そこから妥当に導ける業界・事業上の推定をJSON化してください。
重要な制約:
- 実在がはっきりしない・曖昧な組織名（「共同企業体」「その他」「株式会社」のみ、「○○JV」など）は一切載せない。不明なら省略する。
- 入札・調達・補助金の相手は、共同企業体名ではなく発注元の省庁・自治体・公的機関名を name に書く（例: デジタル庁、厚生労働省）。発注機関が不明ならその件は載せない。
- 取引先の description は具体的な取引内容（推定可）。推定も弱く種別しか分からない場合は空文字（表示は主要取引先）。空の代わりに「主要取引先」と書かない。
- subsidiaries は議決権50%以上の子会社・完全子会社のみ。独立した上場企業は、出資が50%未満なら affiliates、資本関係が無ければ business_partners。
- 出資比率が分かる場合は必ず ratio に入れる（パーセント、例: 100, 51, 5.02）。`
	searchPrompt, parseUser := buildRelationsSearchPrompts(companyName, websiteURL)
	raw, modelsUsed, err := searchThenParse(ctx, f.shared, f.llm, companyName, sharedSearchPrompt(companyName, websiteURL), searchPrompt, systemPrompt, parseUser, 1200)
	if err != nil {
		return nil, fmt.Errorf("企業関係情報のAI取得失敗: %w", err)
	}
	result, err := parseCompanyRelationsResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceWebSearch
	result.SourceURL = strings.TrimSpace(websiteURL)
	result.ModelUsed = modelsUsed
	result.Confidence = companyfetch.ConfidenceMedium
	// 取引内容の追撃は acquireForCompany 側（既存関係マージ後）に集約し、二重 LLM を避ける。
	return result, nil
}

// buildRelationsSearchPrompts は企業関係の検索プロンプトと抽出プロンプトを組み立てる。
// 検索プロンプトはそのまま検索クエリになるので企業名を整え、抽出プロンプトでは囲む（#1970 のレビュー）。
func buildRelationsSearchPrompts(companyName, websiteURL string) (searchPrompt, parseUser string) {
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchPrompt = fmt.Sprintf(
		`日本の企業「%s」%sについて、公開情報から次を調べてください。
1. 子会社・グループ会社（議決権50%%以上）と資本関係の内容。出資比率が分かればパーセントで
2. 資本提携・関連会社（議決権50%%未満の出資）と提携内容
3. 取引先・業務提携先（資本関係が無い相手。独立上場企業の協業含む）と取引内容。曖昧な社名は載せない
4. 入札・調達・補助金がある場合は発注元の省庁・自治体名（共同企業体名は載せない）
5. 上場区分・証券コード
根拠URLがあれば含めてください。組織名が不明・曖昧ならその件は省略してください。`,
		shared.InlinePromptName(companyName), siteHint,
	)
	parseUser = fmt.Sprintf(
		"次の企業について次のJSON形式で回答してください。name ははっきりした組織名のみ（曖昧・JV・その他は禁止）。subsidiaries は議決権50%%以上のみ。50%%未満の出資は affiliates。資本関係が無い相手は business_partners。入札案件は省庁・自治体名。ratio はパーセント。description は取引内容（推定可、弱ければ空）。\n%s%s",
		shared.UntrustedSection(companyName, "企業名"), companyRelationsJSONSchema,
	)
	return searchPrompt, parseUser
}

// enrichTransactionDescriptions は取引先名は分かっているが取引内容が空の件を、取引内容に特化した検索で埋める。
func (f *CompanyRelationsFetcher) enrichTransactionDescriptions(
	ctx context.Context,
	companyName, websiteURL string,
	relations []RelationEntry,
) (*CompanyRelationsResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	targets := make([]string, 0)
	for _, rel := range relations {
		if companyfetch.SanitizeRelationDescription(rel.Description) != "" {
			continue
		}
		name := strings.TrimSpace(rel.Name)
		if name == "" {
			continue
		}
		targets = append(targets, name)
		if len(targets) >= config.RelationEnrichMaxTargets() {
			break
		}
	}
	if len(targets) == 0 {
		return &CompanyRelationsResult{Relations: nil}, nil
	}

	systemPrompt := `あなたは日本企業の取引関係に詳しいアシスタントです。検索結果と妥当な推定をJSON化してください。
はっきりした組織名のみ載せる。曖昧名・共同企業体・「その他」は禁止。入札・調達の相手は省庁・自治体名。不明なら省略。
description は具体的な取引内容（推定可）。弱い場合は空文字（『主要取引先』と書かない）。`
	searchPrompt, parseUser := buildRelationEnrichPrompts(companyName, websiteURL, targets)
	// ここは共有検索を使わない。この補完パスが走るのは、1回目の取得で
	// description が空だった相手が残っているときだけで、その「1回目」が
	// 読んだのが共有テキストそのものだからだ。同じテキストを読み直しても
	// description は埋まらず、補完が丸ごと無意味になる。
	// 技術スタックを統合から外したのと同じ理由(#1124)。
	// 相手を名指しした検索をここで別に撃つ。
	raw, modelsUsed, err := f.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 1000)
	if err != nil {
		return nil, err
	}
	result, err := parseCompanyRelationsResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceWebSearch
	result.ModelUsed = modelsUsed
	result.Confidence = companyfetch.ConfidenceMedium
	return result, nil
}

// buildRelationEnrichPrompts は取引内容の補完に使う検索プロンプトと抽出プロンプトを組み立てる。
// 関連企業名も Web・DB 由来なので、企業名と同じく、検索プロンプトでは1件ずつ整え、
// 抽出プロンプトでは囲む（#1970 のレビュー）。
func buildRelationEnrichPrompts(companyName, websiteURL string, targets []string) (searchPrompt, parseUser string) {
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchTargets := make([]string, 0, len(targets))
	for _, t := range targets {
		if n := shared.InlinePromptName(t); n != "" {
			searchTargets = append(searchTargets, n)
		}
	}
	searchPrompt = fmt.Sprintf(
		`日本の企業「%s」%sと、次の関連企業との関係について、公開情報および事業内容から妥当な取引・協業・資本関係の内容を調べてください: %s。各ペアについて「何を取引・提携しているか」を1行で示してください。根拠URLがあれば含めてください。推定もできないペアは省略するか description を空にしてください。`,
		shared.InlinePromptName(companyName), siteHint, strings.Join(searchTargets, "、"),
	)
	parseSchema := `{
  "business_partners": [{"name": "関連企業名", "description": "取引内容（推定可）。弱い場合は空文字"}],
  "affiliates": [{"name": "関連企業名", "description": "資本・提携の具体内容"}],
  "subsidiaries": [{"name": "関連企業名", "description": "資本関係の具体内容", "ratio": null}],
  "market_info": {"is_listed": false, "market_type": "", "stock_code": ""}
}`
	parseUser = fmt.Sprintf(
		"次の企業と関連企業について次のJSONを返してください。description は具体的な取引内容（推定可）。弱いフォールバックなら空文字（『主要取引先』と書かない）。\n%s%s%s",
		shared.UntrustedSection(companyName, "企業名"), shared.UntrustedSection(strings.Join(targets, "、"), "関連企業"), parseSchema,
	)
	return searchPrompt, parseUser
}

type aiRelationsPayload struct {
	Subsidiaries     []aiRelationName `json:"subsidiaries"`
	Affiliates       []aiRelationName `json:"affiliates"`
	BusinessPartners []aiRelationName `json:"business_partners"`
	MarketInfo       struct {
		IsListed   bool   `json:"is_listed"`
		MarketType string `json:"market_type"`
		StockCode  string `json:"stock_code"`
	} `json:"market_info"`
}

type aiRelationName struct {
	Name        string   `json:"name"`
	Ratio       *float64 `json:"ratio"`
	Description string   `json:"description"`
}

// capitalRelationType は出資比率を優先し、LLM の桶分けを上書きする。
// 50%以上は子会社、0より大きく50未満は関連会社。比率不明なら申告どおり。
func capitalRelationType(claimed string, ratio *float64) string {
	if ratio != nil {
		if *ratio >= 50 {
			return "capital_subsidiary"
		}
		if *ratio > 0 {
			return "capital_affiliate"
		}
	}
	switch claimed {
	case "capital_subsidiary", "capital_affiliate", "business_partner":
		return claimed
	default:
		return "business_partner"
	}
}

func appendParsedRelation(result *CompanyRelationsResult, item aiRelationName, claimed string) {
	name := strings.TrimSpace(item.Name)
	if !companyfetch.IsClearOrganizationName(name) {
		return
	}
	result.Relations = append(result.Relations, RelationEntry{
		Name:         name,
		RelationType: capitalRelationType(claimed, item.Ratio),
		Ratio:        item.Ratio,
		Description:  companyfetch.SanitizeRelationDescription(item.Description),
	})
}

func parseCompanyRelationsResult(text string) (*CompanyRelationsResult, error) {
	obj, err := companyfetch.ExtractJSONObject(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ai response: %w", err)
	}
	var payload aiRelationsPayload
	if err := json.Unmarshal([]byte(obj), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse company relations json: %w", err)
	}

	result := &CompanyRelationsResult{}
	for _, item := range payload.Subsidiaries {
		appendParsedRelation(result, item, "capital_subsidiary")
	}
	for _, item := range payload.Affiliates {
		appendParsedRelation(result, item, "capital_affiliate")
	}
	for _, item := range payload.BusinessPartners {
		appendParsedRelation(result, item, "business_partner")
	}
	if payload.MarketInfo.MarketType != "" || payload.MarketInfo.StockCode != "" || payload.MarketInfo.IsListed {
		result.MarketInfo = &CompanyMarketInfoResult{
			IsListed:   payload.MarketInfo.IsListed,
			MarketType: payload.MarketInfo.MarketType,
			StockCode:  payload.MarketInfo.StockCode,
		}
	}
	return result, nil
}

func relationsNeedDescriptionEnrichment(relations []RelationEntry) bool {
	for _, rel := range relations {
		// 取引関係のみ、取引内容の欠落を再取得トリガーにする（資本関係の空説明は許容）
		if !strings.HasPrefix(rel.RelationType, "business_") {
			continue
		}
		if strings.TrimSpace(rel.Name) == "" {
			continue
		}
		if companyfetch.SanitizeRelationDescription(rel.Description) == "" {
			return true
		}
	}
	return false
}
