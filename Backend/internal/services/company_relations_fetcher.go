package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/gbizinfo"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RelationEntry はプレビュー/確定用の企業関係1件。
type RelationEntry struct {
	Name         string   `json:"name"`
	RelationType string   `json:"relation_type"` // capital_subsidiary, capital_affiliate, business_partner
	Ratio        *float64 `json:"ratio,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// CompanyMarketInfoResult は市場情報の取得結果。
type CompanyMarketInfoResult struct {
	IsListed   bool   `json:"is_listed"`
	MarketType string `json:"market_type"` // prime, standard, growth, unlisted
	StockCode  string `json:"stock_code,omitempty"`
}

// CompanyRelationsResult は企業関係・市場情報の取得結果。
type CompanyRelationsResult struct {
	Relations  []RelationEntry          `json:"relations"`
	MarketInfo *CompanyMarketInfoResult `json:"market_info,omitempty"`
	Source     string                   `json:"source,omitempty"`
	SourceURL  string                   `json:"source_url,omitempty"`
	ModelUsed  string                   `json:"model_used,omitempty"`
	Confidence string                   `json:"confidence,omitempty"`
	SavedCount int                      `json:"saved_count,omitempty"`
	// Phase 3 運用メタ: TTL / 予算ガードで Search をスキップしたとき
	FromCache      bool   `json:"from_cache,omitempty"`
	BudgetExceeded bool   `json:"budget_exceeded,omitempty"`
	SkipReason     string `json:"skip_reason,omitempty"` // ttl | budget
}

// CompanyRelationsFetcher は gBizINFO を優先し、不足分のみ AI Search で関係・市場情報を取得する。
type CompanyRelationsFetcher struct {
	companyRepo  repository.CompanyRepository
	relationRepo repository.CompanyRelationRepository
	llm          *companyfetch.LLM
	gbiz         *gbizinfo.GBizInfoService
	flight       *CompanySearchFlight
}

func NewCompanyRelationsFetcher(
	companyRepo repository.CompanyRepository,
	relationRepo repository.CompanyRelationRepository,
	client *openai.Client,
	gbiz ...*gbizinfo.GBizInfoService,
) *CompanyRelationsFetcher {
	f := &CompanyRelationsFetcher{
		companyRepo:  companyRepo,
		relationRepo: relationRepo,
		llm:          companyfetch.NewLLM(client),
	}
	if len(gbiz) > 0 {
		f.gbiz = gbiz[0]
	}
	return f
}

func (f *CompanyRelationsFetcher) SetSearchBudget(budget companyfetch.SearchBudget) {
	if f == nil {
		return
	}
	if f.llm == nil {
		f.llm = &companyfetch.LLM{}
	}
	f.llm.Budget = budget
}

func (f *CompanyRelationsFetcher) SetSearchFlight(flight *CompanySearchFlight) {
	if f != nil {
		f.flight = flight
	}
}

const companyRelationsJSONSchema = `{
  "subsidiaries": [{"name": "子会社・グループ会社名", "ratio": 出資比率（不明なら省略）, "description": "資本関係の具体内容（例: 完全子会社・議決権○%）"}],
  "affiliates": [{"name": "資本提携・関連会社名", "description": "資本関係の具体内容（例: 資本業務提携・出資比率○%）"}],
  "business_partners": [{"name": "はっきりした取引先名または省庁・自治体名（曖昧・JV・その他は禁止）", "description": "取引内容。公開事実が無くても業界・事業内容から妥当に推定できる内容を書いてよい。推定もできず種別しか分からない場合は空文字（表示は主要取引先）"}],
  "market_info": {
    "is_listed": true/false,
    "market_type": "prime|standard|growth|unlisted",
    "stock_code": "証券コード（4桁、非上場なら空文字）"
  }
}`

// FetchAndSave は指定企業の関係・市場情報を取得して DB に保存する。
func (f *CompanyRelationsFetcher) FetchAndSave(ctx context.Context, companyID uint, forceRefresh bool) (*CompanyRelationsResult, error) {
	company, err := f.companyRepo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	if !forceRefresh && companyfetch.IsFresh(company.RelationsFetchedAt, companyfetch.TTLRelations) {
		result, err := f.resultFromDB(company, companyfetch.SourceGBiz, "gbizinfo+cache", companyfetch.ConfidenceHigh)
		if err != nil {
			return nil, err
		}
		// 取引内容が空でも「主要取引先」フォールバックとして確定済みなら TTL 内は再取得しない。
		// 推定し直したい場合は force=true。
		if relationsResultHasData(result) {
			return markRelationsCacheSkip(result, "ttl", false), nil
		}
	}

	run := func() (any, error) {
		return f.acquireForCompany(ctx, company)
	}
	var result *CompanyRelationsResult
	if f.flight != nil {
		v, ferr := f.flight.Do("relations", normalizeCompanyKey(company.Name), run)
		err = ferr
		if v != nil {
			result, _ = v.(*CompanyRelationsResult)
		}
	} else {
		result, err = f.acquireForCompany(ctx, company)
	}
	if err != nil {
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			cached, cerr := f.resultFromDB(company, companyfetch.SourceGBiz, "cache", companyfetch.ConfidenceMedium)
			if cerr != nil {
				return nil, cerr
			}
			return markRelationsCacheSkip(cached, "budget", true), nil
		}
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("company relations acquire returned nil")
	}

	saved, err := f.persistResult(company, result)
	if err != nil {
		return nil, err
	}
	result.SavedCount = saved

	// AI が関係を返しても永続化が全失敗なら取得済みにしない。
	if saved > 0 {
		now := time.Now()
		company.RelationsFetchedAt = &now
		if err := f.companyRepo.Update(company); err != nil {
			return nil, fmt.Errorf("failed to update company: %w", err)
		}
	}
	return result, nil
}

// LoadSaved は DB に保存済みの関係・市場情報だけを返す（AI 再取得なし）。
func (f *CompanyRelationsFetcher) LoadSaved(companyID uint) (*CompanyRelationsResult, error) {
	company, err := f.companyRepo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}
	return f.resultFromDB(company, companyfetch.SourceGBiz, "db", companyfetch.ConfidenceHigh)
}

// ConfirmAndSave はプレビュー済みの関係・市場情報を LLM 再実行なしで DB に確定保存する。
func (f *CompanyRelationsFetcher) ConfirmAndSave(companyID uint, result *CompanyRelationsResult) (*CompanyRelationsResult, error) {
	if result == nil {
		return nil, fmt.Errorf("result is required")
	}
	company, err := f.companyRepo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	saved, err := f.persistResult(company, result)
	if err != nil {
		return nil, err
	}

	out := *result
	out.SavedCount = saved
	if saved > 0 {
		now := time.Now()
		company.RelationsFetchedAt = &now
		if err := f.companyRepo.Update(company); err != nil {
			return nil, fmt.Errorf("failed to update company: %w", err)
		}
	}
	return &out, nil
}

// Acquire は企業名から関係・市場情報をプレビュー取得する（DB 非更新）。
func (f *CompanyRelationsFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*CompanyRelationsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	result, err := f.acquireViaAISearch(ctx, companyName, websiteURL)
	if err != nil {
		return nil, err
	}
	if result != nil && relationsNeedDescriptionEnrichment(result.Relations) {
		if enriched, eerr := f.enrichTransactionDescriptions(ctx, companyName, websiteURL, result.Relations); eerr == nil && enriched != nil {
			result = mergeRelationsResult(result.Relations, result.MarketInfo, enriched)
			if enriched.ModelUsed != "" {
				result.ModelUsed = result.ModelUsed + "+desc:" + enriched.ModelUsed
			}
			result.Source = companyfetch.SourceWebSearch
			result.Confidence = companyfetch.ConfidenceMedium
		}
	}
	return result, nil
}

func (f *CompanyRelationsFetcher) acquireForCompany(ctx context.Context, company *models.Company) (*CompanyRelationsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	var existingEntries []RelationEntry
	if f.relationRepo != nil {
		relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
		if err != nil {
			return nil, err
		}
		existingEntries = relationsToEntries(company.ID, relations)
	}

	source := companyfetch.SourceGBiz
	confidence := companyfetch.ConfidenceHigh
	modelUsed := "gbizinfo"

	if f.gbiz != nil && strings.TrimSpace(company.CorporateNumber) != "" {
		if _, err := f.gbiz.SyncCompany(ctx, company.ID); err == nil {
			relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
			if err != nil {
				return nil, err
			}
			existingEntries = relationsToEntries(company.ID, relations)
		}
	}

	var marketInfo *CompanyMarketInfoResult
	if f.relationRepo != nil {
		if info, err := f.relationRepo.GetMarketInfoByCompanyID(company.ID); err == nil && info != nil {
			marketInfo = marketInfoFromModel(info)
		}
	}

	// 取引先名があっても取引内容が空なら AI で補完する
	needsAI := len(existingEntries) == 0 || marketInfo == nil || marketInfo.StockCode == "" ||
		relationsNeedDescriptionEnrichment(existingEntries)
	if !needsAI {
		return f.buildResultFromExisting(company, nil, marketInfo, source, modelUsed, confidence)
	}

	ai, err := f.acquireViaAISearch(ctx, company.Name, company.WebsiteURL)
	if err != nil {
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			return nil, err
		}
		if len(existingEntries) > 0 || marketInfo != nil {
			return f.buildResultFromExisting(company, nil, marketInfo, source, modelUsed, confidence)
		}
		return nil, err
	}

	merged := mergeRelationsResult(existingEntries, marketInfo, ai)
	if ai.ModelUsed != "" {
		modelUsed = source + "+" + ai.ModelUsed
	}
	if len(existingEntries) > 0 {
		source = companyfetch.SourceGBiz + "+" + companyfetch.SourceWebSearch
		confidence = companyfetch.ConfidenceMedium
	} else {
		source = ai.Source
		confidence = ai.Confidence
		modelUsed = ai.ModelUsed
	}

	// まだ取引内容が空の取引先があれば、取引内容に特化した追撃検索
	if relationsNeedDescriptionEnrichment(merged.Relations) {
		if enriched, eerr := f.enrichTransactionDescriptions(ctx, company.Name, company.WebsiteURL, merged.Relations); eerr == nil && enriched != nil {
			merged = mergeRelationsResult(merged.Relations, merged.MarketInfo, enriched)
			if enriched.ModelUsed != "" {
				modelUsed = modelUsed + "+desc:" + enriched.ModelUsed
			}
		}
	}

	merged.Source = source
	merged.ModelUsed = modelUsed
	merged.Confidence = confidence
	merged.SourceURL = firstNonEmpty(company.WebsiteURL, ai.SourceURL)
	return merged, nil
}

func (f *CompanyRelationsFetcher) acquireViaAISearch(ctx context.Context, companyName, websiteURL string) (*CompanyRelationsResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	systemPrompt := `あなたは日本企業の資本関係・取引関係・上場情報に詳しいアシスタントです。検索結果と、そこから妥当に導ける業界・事業上の推定をJSON化してください。
重要な制約:
- 実在がはっきりしない・曖昧な組織名（「共同企業体」「その他」「株式会社」のみ、「○○JV」など）は一切載せない。不明なら省略する。
- 入札・調達・補助金の相手は、共同企業体名ではなく発注元の省庁・自治体・公的機関名を name に書く（例: デジタル庁、厚生労働省）。発注機関が不明ならその件は載せない。
- 取引先の description は具体的な取引内容（推定可）。推定も弱く種別しか分からない場合は空文字（表示は主要取引先）。空の代わりに「主要取引先」と書かない。`
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」%sについて、公開情報から次を調べてください。
1. 子会社・グループ会社と資本関係の内容（はっきりした社名のみ）
2. 資本提携・関連会社と提携内容（はっきりした社名のみ）
3. 取引先（顧客・仕入先・業務提携先など）と取引内容。曖昧な社名は載せない
4. 入札・調達・補助金がある場合は発注元の省庁・自治体名（共同企業体名は載せない）
5. 上場区分・証券コード
根拠URLがあれば含めてください。組織名が不明・曖昧ならその件は省略してください。`,
		companyName, siteHint,
	)
	parseUser := fmt.Sprintf(
		"企業名「%s」について次のJSON形式で回答してください。name ははっきりした組織名のみ（曖昧・JV・その他は禁止）。入札案件は省庁・自治体名。description は取引内容（推定可、弱ければ空）。\n%s",
		companyName, companyRelationsJSONSchema,
	)
	raw, modelsUsed, err := f.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 1200)
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

	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	partnerList := strings.Join(targets, "、")
	systemPrompt := `あなたは日本企業の取引関係に詳しいアシスタントです。検索結果と妥当な推定をJSON化してください。
はっきりした組織名のみ載せる。曖昧名・共同企業体・「その他」は禁止。入札・調達の相手は省庁・自治体名。不明なら省略。
description は具体的な取引内容（推定可）。弱い場合は空文字（『主要取引先』と書かない）。`
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」%sと、次の関連企業との関係について、公開情報および事業内容から妥当な取引・協業・資本関係の内容を調べてください: %s。各ペアについて「何を取引・提携しているか」を1行で示してください。根拠URLがあれば含めてください。推定もできないペアは省略するか description を空にしてください。`,
		companyName, siteHint, partnerList,
	)
	parseSchema := `{
  "business_partners": [{"name": "関連企業名", "description": "取引内容（推定可）。弱い場合は空文字"}],
  "affiliates": [{"name": "関連企業名", "description": "資本・提携の具体内容"}],
  "subsidiaries": [{"name": "関連企業名", "description": "資本関係の具体内容", "ratio": null}],
  "market_info": {"is_listed": false, "market_type": "", "stock_code": ""}
}`
	parseUser := fmt.Sprintf(
		"企業「%s」と関連企業（%s）について次のJSONを返してください。description は具体的な取引内容（推定可）。弱いフォールバックなら空文字（『主要取引先』と書かない）。\n%s",
		companyName, partnerList, parseSchema,
	)
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

func (f *CompanyRelationsFetcher) resultFromDB(company *models.Company, source, model, confidence string) (*CompanyRelationsResult, error) {
	existingNames := map[string]struct{}{}
	if f.relationRepo != nil {
		relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
		if err != nil {
			return nil, err
		}
		for _, rel := range relations {
			for _, name := range relationPartnerNames(company.ID, rel) {
				existingNames[normalizeRelationName(name)] = struct{}{}
			}
		}
	}
	var marketInfo *CompanyMarketInfoResult
	if f.relationRepo != nil {
		if info, err := f.relationRepo.GetMarketInfoByCompanyID(company.ID); err == nil && info != nil {
			marketInfo = marketInfoFromModel(info)
		}
	}
	return f.buildResultFromExisting(company, existingNames, marketInfo, source, model, confidence)
}

func (f *CompanyRelationsFetcher) buildResultFromExisting(
	company *models.Company,
	_ map[string]struct{},
	marketInfo *CompanyMarketInfoResult,
	source, modelUsed, confidence string,
) (*CompanyRelationsResult, error) {
	if f.relationRepo == nil {
		return &CompanyRelationsResult{
			Relations:  nil,
			MarketInfo: marketInfo,
			Source:     source,
			ModelUsed:  modelUsed,
			Confidence: confidence,
		}, nil
	}
	relations, err := f.relationRepo.GetRelationsByCompanyID(company.ID)
	if err != nil {
		return nil, err
	}
	entries := relationsToEntries(company.ID, relations)
	return &CompanyRelationsResult{
		Relations:  entries,
		MarketInfo: marketInfo,
		Source:     source,
		SourceURL:  company.WebsiteURL,
		ModelUsed:  modelUsed,
		Confidence: confidence,
	}, nil
}

func (f *CompanyRelationsFetcher) persistResult(company *models.Company, result *CompanyRelationsResult) (int, error) {
	if f.relationRepo == nil {
		return 0, fmt.Errorf("relation repository is nil")
	}
	saved := 0
	now := time.Now()
	sourceTag := result.Source
	if sourceTag == "" {
		sourceTag = companyfetch.SourceWebSearch
	}

	for _, entry := range result.Relations {
		name := strings.TrimSpace(entry.Name)
		if !companyfetch.IsClearOrganizationName(name) {
			continue
		}
		toCompany, err := f.companyRepo.FindByName(name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if toCompany == nil {
			toCompany = &models.Company{
				Name:            name,
				SourceType:      sourceTag,
				SourceFetchedAt: &now,
				IsProvisional:   true,
				DataStatus:      "draft",
			}
			if err := f.companyRepo.Create(toCompany); err != nil {
				continue
			}
		}
		// 種別ラベル（主要取引先など）は保存せず、具体的な取引内容のみ残す
		desc := companyfetch.SanitizeRelationDescription(entry.Description)
		var upsertErr error
		if models.IsCapitalRelationType(entry.RelationType) {
			ratio := entry.Ratio
			if ratio == nil && entry.RelationType == "capital_subsidiary" {
				r := 100.0
				ratio = &r
			}
			upsertErr = f.relationRepo.UpsertCapitalRelation(company.ID, toCompany.ID, entry.RelationType, ratio, desc)
		} else {
			relationType := entry.RelationType
			if relationType == "" {
				relationType = "business_partner"
			}
			upsertErr = f.relationRepo.UpsertBusinessRelation(company.ID, toCompany.ID, relationType, desc)
		}
		if upsertErr != nil {
			continue
		}
		saved++
	}

	if result.MarketInfo != nil && companyfetch.HasMeaningfulMarketInfo(
		result.MarketInfo.IsListed, result.MarketInfo.MarketType, result.MarketInfo.StockCode,
	) {
		marketType := normalizeMarketType(result.MarketInfo.MarketType, result.MarketInfo.IsListed)
		info := &models.CompanyMarketInfo{
			CompanyID:  company.ID,
			MarketType: marketType,
			IsListed:   result.MarketInfo.IsListed || marketType != "unlisted",
			StockCode:  strings.TrimSpace(result.MarketInfo.StockCode),
		}
		if err := f.relationRepo.UpsertMarketInfo(info); err == nil {
			saved++
		}
	}
	return saved, nil
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
		if name := strings.TrimSpace(item.Name); companyfetch.IsClearOrganizationName(name) {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "capital_subsidiary",
				Ratio:        item.Ratio,
				Description:  companyfetch.SanitizeRelationDescription(item.Description),
			})
		}
	}
	for _, item := range payload.Affiliates {
		if name := strings.TrimSpace(item.Name); companyfetch.IsClearOrganizationName(name) {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "capital_affiliate",
				Description:  companyfetch.SanitizeRelationDescription(item.Description),
			})
		}
	}
	for _, item := range payload.BusinessPartners {
		if name := strings.TrimSpace(item.Name); companyfetch.IsClearOrganizationName(name) {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "business_partner",
				Description:  companyfetch.SanitizeRelationDescription(item.Description),
			})
		}
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

func mergeRelationsResult(
	existing []RelationEntry,
	baseMarket *CompanyMarketInfoResult,
	ai *CompanyRelationsResult,
) *CompanyRelationsResult {
	out := &CompanyRelationsResult{}
	byKey := map[string]RelationEntry{}
	order := make([]string, 0)

	addOrMerge := func(entry RelationEntry) {
		name := strings.TrimSpace(entry.Name)
		if !companyfetch.IsClearOrganizationName(name) {
			return
		}
		entry.Name = name
		entry.Description = companyfetch.SanitizeRelationDescription(entry.Description)
		key := normalizeRelationName(name) + "|" + entry.RelationType
		if prev, ok := byKey[key]; ok {
			if prev.Description == "" && entry.Description != "" {
				prev.Description = entry.Description
			}
			if prev.Ratio == nil && entry.Ratio != nil {
				prev.Ratio = entry.Ratio
			}
			byKey[key] = prev
			return
		}
		// 同名で種別違いがある場合、空の説明だけ AI 内容で補完
		norm := normalizeRelationName(name)
		if entry.Description != "" {
			for k, prev := range byKey {
				if normalizeRelationName(prev.Name) != norm {
					continue
				}
				if prev.Description == "" {
					prev.Description = entry.Description
					byKey[k] = prev
				}
			}
		}
		byKey[key] = entry
		order = append(order, key)
	}

	for _, entry := range existing {
		addOrMerge(entry)
	}
	if ai != nil {
		for _, entry := range ai.Relations {
			addOrMerge(entry)
		}
	}
	for _, key := range order {
		out.Relations = append(out.Relations, byKey[key])
	}
	// order に無い（AI 新規のみで addOrMerge が order に追加済み）— already in order via addOrMerge
	// ただし既存に無く AI で新規追加されたものは order に入る。OK

	if baseMarket != nil {
		out.MarketInfo = baseMarket
	} else if ai != nil {
		out.MarketInfo = ai.MarketInfo
	}
	if out.MarketInfo == nil && ai != nil {
		out.MarketInfo = ai.MarketInfo
	}
	if baseMarket != nil && ai != nil && ai.MarketInfo != nil {
		merged := *baseMarket
		if merged.StockCode == "" {
			merged.StockCode = ai.MarketInfo.StockCode
		}
		if merged.MarketType == "" {
			merged.MarketType = ai.MarketInfo.MarketType
		}
		if !merged.IsListed && ai.MarketInfo.IsListed {
			merged.IsListed = ai.MarketInfo.IsListed
		}
		out.MarketInfo = &merged
	}
	return out
}

func relationsToEntries(companyID uint, relations []models.CompanyRelation) []RelationEntry {
	entries := make([]RelationEntry, 0, len(relations))
	for _, rel := range relations {
		name, relationType := relationEntryFromModel(companyID, rel)
		if name == "" {
			continue
		}
		entries = append(entries, RelationEntry{
			Name:         name,
			RelationType: relationType,
			Ratio:        rel.Ratio,
			Description:  companyfetch.SanitizeRelationDescription(rel.Description),
		})
	}
	return entries
}

func relationEntryFromModel(companyID uint, rel models.CompanyRelation) (name, relationType string) {
	relationType = rel.RelationType
	if models.IsCapitalRelationType(relationType) {
		if rel.ParentID != nil && *rel.ParentID == companyID && rel.Child != nil {
			return rel.Child.Name, relationType
		}
		if rel.ChildID != nil && *rel.ChildID == companyID && rel.Parent != nil {
			return rel.Parent.Name, relationType
		}
		return "", relationType
	}
	if rel.FromID != nil && *rel.FromID == companyID && rel.To != nil {
		return rel.To.Name, relationType
	}
	if rel.ToID != nil && *rel.ToID == companyID && rel.From != nil {
		return rel.From.Name, relationType
	}
	return "", relationType
}

func relationPartnerNames(companyID uint, rel models.CompanyRelation) []string {
	name, _ := relationEntryFromModel(companyID, rel)
	if name == "" {
		return nil
	}
	return []string{name}
}

func marketInfoFromModel(info *models.CompanyMarketInfo) *CompanyMarketInfoResult {
	if info == nil {
		return nil
	}
	return &CompanyMarketInfoResult{
		IsListed:   info.IsListed,
		MarketType: info.MarketType,
		StockCode:  info.StockCode,
	}
}

func normalizeRelationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeMarketType(marketType string, isListed bool) string {
	m := strings.ToLower(strings.TrimSpace(marketType))
	switch m {
	case "prime", "standard", "growth", "unlisted":
		return m
	}
	if isListed {
		return "standard"
	}
	return "unlisted"
}

func markRelationsCacheSkip(r *CompanyRelationsResult, reason string, budgetExceeded bool) *CompanyRelationsResult {
	if r == nil {
		r = &CompanyRelationsResult{}
	}
	r.FromCache = true
	r.SkipReason = reason
	r.BudgetExceeded = budgetExceeded
	return r
}

// HasStoredData は DB に関係、または確定した市場情報（上場・非上場）があるか。
func (f *CompanyRelationsFetcher) HasStoredData(companyID uint) bool {
	if f == nil || f.relationRepo == nil {
		return false
	}
	rels, err := f.relationRepo.GetRelationsByCompanyID(companyID)
	if err == nil && len(rels) > 0 {
		return true
	}
	info, err := f.relationRepo.GetMarketInfoByCompanyID(companyID)
	if err != nil || info == nil {
		return false
	}
	return companyfetch.HasMeaningfulMarketInfo(info.IsListed, info.MarketType, info.StockCode)
}

func relationsResultHasData(r *CompanyRelationsResult) bool {
	if r == nil {
		return false
	}
	if len(r.Relations) > 0 {
		return true
	}
	if r.SavedCount > 0 {
		// SavedCount は meaningful market / 関係の upsert 成功数のみを数える
		return true
	}
	if r.MarketInfo == nil {
		return false
	}
	return companyfetch.HasMeaningfulMarketInfo(
		r.MarketInfo.IsListed, r.MarketInfo.MarketType, r.MarketInfo.StockCode,
	)
}
