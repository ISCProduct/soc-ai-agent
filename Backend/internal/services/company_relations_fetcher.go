package services

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
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
	gbiz         *GBizInfoService
	flight       *CompanySearchFlight
}

func NewCompanyRelationsFetcher(
	companyRepo repository.CompanyRepository,
	relationRepo repository.CompanyRelationRepository,
	client *openai.Client,
	gbiz ...*GBizInfoService,
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
  "subsidiaries": [{"name": "子会社・グループ会社名", "ratio": 出資比率（不明なら省略）, "description": "関係の説明（例: 完全子会社）"}],
  "affiliates": [{"name": "資本提携・関連会社名", "description": "関係の説明（例: 資本提携）"}],
  "business_partners": [{"name": "主要取引先名", "description": "取引内容の1行説明（例: クラウド基盤の共同開発）"}],
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
		// 関係・市場が空なら TTL 内でも再取得する
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
	return f.acquireViaAISearch(ctx, companyName, websiteURL)
}

func (f *CompanyRelationsFetcher) acquireForCompany(ctx context.Context, company *models.Company) (*CompanyRelationsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

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

	source := companyfetch.SourceGBiz
	confidence := companyfetch.ConfidenceHigh
	modelUsed := "gbizinfo"

	if f.gbiz != nil && strings.TrimSpace(company.CorporateNumber) != "" {
		if _, err := f.gbiz.SyncCompany(ctx, company.ID); err == nil {
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
	}

	var marketInfo *CompanyMarketInfoResult
	if f.relationRepo != nil {
		if info, err := f.relationRepo.GetMarketInfoByCompanyID(company.ID); err == nil && info != nil {
			marketInfo = marketInfoFromModel(info)
		}
	}

	needsAI := len(existingNames) == 0 || marketInfo == nil || marketInfo.StockCode == ""
	if !needsAI {
		return f.buildResultFromExisting(company, existingNames, marketInfo, source, modelUsed, confidence)
	}

	ai, err := f.acquireViaAISearch(ctx, company.Name, company.WebsiteURL)
	if err != nil {
		// 予算超過は呼び出し元でキャッシュ継続＋メタ付与するため伝播する
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			return nil, err
		}
		if len(existingNames) > 0 || marketInfo != nil {
			return f.buildResultFromExisting(company, existingNames, marketInfo, source, modelUsed, confidence)
		}
		return nil, err
	}

	merged := mergeRelationsResult(existingNames, marketInfo, ai)
	if ai.ModelUsed != "" {
		modelUsed = source + "+" + ai.ModelUsed
	}
	if len(existingNames) > 0 {
		source = companyfetch.SourceGBiz + "+" + companyfetch.SourceWebSearch
		confidence = companyfetch.ConfidenceMedium
	} else {
		source = ai.Source
		confidence = ai.Confidence
		modelUsed = ai.ModelUsed
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

	systemPrompt := `あなたは日本企業の資本関係・取引関係・上場情報に詳しいアシスタントです。検索結果に明示された事実のみをJSON化してください。推測で企業名を作らないでください。`
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」%sについて、公開情報から確認できる子会社・グループ会社・資本提携先・主要取引先・上場区分・証券コードを調べてください。各事実の根拠URLを含めてください。不明な項目は推測せず空配列または空文字にしてください。`,
		companyName, siteHint,
	)
	parseUser := fmt.Sprintf(
		"企業名「%s」について、検索結果の事実のみに基づき次のJSON形式で回答してください。検索結果に無い項目は空配列または空文字。\n%s",
		companyName, companyRelationsJSONSchema,
	)
	raw, modelsUsed, err := f.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 800)
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
	existingNames map[string]struct{},
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
		if name == "" {
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
		desc := companyfetch.NormalizeRelationDescription(entry.Description, entry.RelationType)
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
		if name := strings.TrimSpace(item.Name); name != "" {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "capital_subsidiary",
				Ratio:        item.Ratio,
				Description:  strings.TrimSpace(item.Description),
			})
		}
	}
	for _, item := range payload.Affiliates {
		if name := strings.TrimSpace(item.Name); name != "" {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "capital_affiliate",
				Description:  strings.TrimSpace(item.Description),
			})
		}
	}
	for _, item := range payload.BusinessPartners {
		if name := strings.TrimSpace(item.Name); name != "" {
			result.Relations = append(result.Relations, RelationEntry{
				Name:         name,
				RelationType: "business_partner",
				Description:  strings.TrimSpace(item.Description),
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

func mergeRelationsResult(
	existingNames map[string]struct{},
	baseMarket *CompanyMarketInfoResult,
	ai *CompanyRelationsResult,
) *CompanyRelationsResult {
	out := &CompanyRelationsResult{}
	seen := map[string]struct{}{}
	for key := range existingNames {
		seen[key] = struct{}{}
	}

	if ai != nil {
		for _, entry := range ai.Relations {
			key := normalizeRelationName(entry.Name) + "|" + entry.RelationType
			if _, ok := seen[key]; ok {
				continue
			}
			norm := normalizeRelationName(entry.Name)
			if _, ok := existingNames[norm]; ok {
				continue
			}
			out.Relations = append(out.Relations, entry)
			seen[key] = struct{}{}
		}
	}

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
			Description:  rel.Description,
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

// HasStoredData は DB に関係、または実質的な市場情報があるか。
// market_type=unlisted のみは「データあり」とみなさない。
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
