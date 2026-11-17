package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/gbizinfo"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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
	shared       *SearchContext
	infoFetcher  *CompanyInfoFetcher
}

// SetInfoFetcher は関連企業として新規作成した会社の詳細情報(業種・住所・URL等)を
// 埋めるためのfetcherを注入する(未設定なら空データのまま作成される)。
func (f *CompanyRelationsFetcher) SetInfoFetcher(infoFetcher *CompanyInfoFetcher) {
	if f != nil {
		f.infoFetcher = infoFetcher
	}
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

// SetSharedSearch は企業ごとに1回だけ行う検索の結果を共有する入れ物を注入する(#1124)。
func (f *CompanyRelationsFetcher) SetSharedSearch(sc *SearchContext) {
	if f != nil {
		f.shared = sc
	}
}

func (f *CompanyRelationsFetcher) SetSearchFlight(flight *CompanySearchFlight) {
	if f != nil {
		f.flight = flight
	}
}

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

	saved, err := f.persistResult(ctx, company, result)
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
func (f *CompanyRelationsFetcher) ConfirmAndSave(ctx context.Context, companyID uint, result *CompanyRelationsResult) (*CompanyRelationsResult, error) {
	if result == nil {
		return nil, fmt.Errorf("result is required")
	}
	company, err := f.companyRepo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	saved, err := f.persistResult(ctx, company, result)
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
