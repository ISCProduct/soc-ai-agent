package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/gbizinfo"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// CompanyInfoResult は企業基本情報の取得結果。
type CompanyInfoResult struct {
	Description        string `json:"description"`
	Industry           string `json:"industry"`
	Location           string `json:"location"`
	WebsiteURL         string `json:"website_url"`
	FoundedYear        int    `json:"founded_year"`
	EmployeeCount      int    `json:"employee_count"`
	EmployeeCountBasis string `json:"employee_count_basis,omitempty"` // consolidated|standalone
	MainBusiness       string `json:"main_business"`
	Culture            string `json:"culture"`
	WorkStyle          string `json:"work_style"`
	TechStack          string `json:"tech_stack,omitempty"`
	WelfareDetails     string `json:"welfare_details,omitempty"`
	Source             string `json:"source,omitempty"`
	SourceURL          string `json:"source_url,omitempty"`
	ModelUsed          string `json:"model_used,omitempty"`
	Confidence         string `json:"confidence,omitempty"`
	// Phase 3 運用メタ: TTL / 予算ガードで Search をスキップしたとき
	FromCache      bool   `json:"from_cache,omitempty"`
	BudgetExceeded bool   `json:"budget_exceeded,omitempty"`
	SkipReason     string `json:"skip_reason,omitempty"` // ttl | budget
}

// CompanyInfoFetcher は gBizINFO を足がかりにしつつ、不足分は AI（安価 Search→Parse）で充足する。
// 高額な deep search は使わない。非上場など gBiz で足りない企業を AI 取得で補う。
type CompanyInfoFetcher struct {
	repo   repository.CompanyRepository
	llm    *companyfetch.LLM
	gbiz   *gbizinfo.GBizInfoService
	flight *CompanySearchFlight

	// provisionFailures は ProvisionByName が失敗した企業名の記録（#1124）。
	// 打ち間違えを繰り返し投げられたときに毎回 Web検索を払わないためのネガティブキャッシュ。
	provisionMu       sync.RWMutex
	provisionFailures map[string]time.Time
}

func NewCompanyInfoFetcher(repo repository.CompanyRepository, client *openai.Client, gbiz ...*gbizinfo.GBizInfoService) *CompanyInfoFetcher {
	f := &CompanyInfoFetcher{repo: repo, llm: companyfetch.NewLLM(client)}
	if len(gbiz) > 0 {
		f.gbiz = gbiz[0]
	}
	return f
}

// SetSearchBudget は月次 Search 予算ガードを注入する。
func (f *CompanyInfoFetcher) SetSearchBudget(budget companyfetch.SearchBudget) {
	if f == nil {
		return
	}
	if f.llm == nil {
		f.llm = &companyfetch.LLM{}
	}
	f.llm.Budget = budget
}

// SetSearchFlight は企業キー単位の singleflight を注入する。
func (f *CompanyInfoFetcher) SetSearchFlight(flight *CompanySearchFlight) {
	if f != nil {
		f.flight = flight
	}
}

const companyInfoJSONSchema = `{
  "description": "企業概要（100〜200文字程度）",
  "industry": "業種（例: IT・ソフトウェア, 金融, 製造業）",
  "location": "本社所在地（例: 東京都渋谷区）",
  "website_url": "公式サイトURL（https://から始まる）",
  "founded_year": 設立年（整数、不明なら0）,
  "employee_count": 直近有価証券報告書の連結従業員数（整数。非上場で連結が無いときだけ単体。不明なら0）,
  "employee_count_basis": "consolidated（連結）または standalone（単体）",
  "main_business": "主要事業内容（50〜100文字程度）",
  "culture": "企業文化・働き方の特徴（50〜100文字程度）",
  "work_style": "勤務スタイル（リモート / ハイブリッド / オフィス のいずれか、不明なら空文字）",
  "tech_stack": "主要技術スタック（カンマ区切り。不明なら空文字）",
  "welfare_details": "福利厚生の要点（不明なら空文字）"
}`

// FetchAndSave は指定企業の基本情報を返す。
// forceRefresh=false かつ InfoFetchedAt が TTL 内なら AI を呼ばずに既存データを返す。
func (f *CompanyInfoFetcher) FetchAndSave(ctx context.Context, companyID uint, forceRefresh bool) (*CompanyInfoResult, error) {
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	// TTL 内でも概要+公式URLが揃っていなければ再取得する（スタンプだけ進んだ残骸・FE不足表示との整合）。
	// 技術取得と同様。強制更新は force。
	if !forceRefresh && companyfetch.IsFresh(company.InfoFetchedAt, companyfetch.TTLInfo) &&
		companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
		return markInfoCacheSkip(companyInfoFromModel(company), "ttl", false), nil
	}

	run := func() (any, error) {
		return f.acquireForCompany(ctx, company)
	}
	var result *CompanyInfoResult
	if f.flight != nil {
		v, ferr := f.flight.Do("info", normalizeCompanyKey(company.Name), run)
		err = ferr
		if v != nil {
			result, _ = v.(*CompanyInfoResult)
		}
	} else {
		result, err = f.acquireForCompany(ctx, company)
	}
	if err != nil {
		if errors.Is(err, companyfetch.ErrSearchBudgetExceeded) {
			// 超過時はキャッシュ（既存DB）のみで継続
			return markInfoCacheSkip(companyInfoFromModel(company), "budget", true), nil
		}
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("company info acquire returned nil")
	}

	applyCompanyInfoResult(company, result)
	now := time.Now()
	company.SourceType = result.Source
	// Search フォールバック時も SourceURL を明示更新し、旧スクレイプ URL の残存を防ぐ
	company.SourceURL = result.SourceURL
	company.SourceFetchedAt = &now
	// 更新後の company（今回の取得結果＋既存の URL/所在地など）で判定する。
	// 手がかりがあれば疎データでもスタンプする（取得試行の記録）。
	// ただし再取得スキップは HasBasicInfo（概要+公式URL）揃い時のみ（FetchAndSave 先頭参照）。
	if companyfetch.HasBasicInfoFootprint(company.Description, company.WebsiteURL, company.Location) {
		company.InfoFetchedAt = &now
	}
	company.LastModelUsed = result.ModelUsed
	company.LastFetchConfidence = result.Confidence

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}
	return result, nil
}

// FetchAndSaveDetails は FetchAndSave の結果を捨てるアダプタ。
// gbizinfo.CompanyDetailFetcher インターフェースを満たすために用意している
// (gbizinfoパッケージはcompanyパッケージをimportできないため、company側から
// このシグネチャで注入する)。
func (f *CompanyInfoFetcher) FetchAndSaveDetails(ctx context.Context, companyID uint) error {
	_, err := f.FetchAndSave(ctx, companyID, false)
	return err
}

// ConfirmAndSave はプレビュー済みの構造化結果を LLM 再実行なしで DB に確定保存する。
// info_fetched_at / last_model_used / last_fetch_confidence も同時更新する。
func (f *CompanyInfoFetcher) ConfirmAndSave(companyID uint, result *CompanyInfoResult) (*CompanyInfoResult, error) {
	if result == nil {
		return nil, fmt.Errorf("result is required")
	}
	company, err := f.repo.FindByID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company not found: %w", err)
	}

	applyCompanyInfoResult(company, result)
	now := time.Now()
	if result.Source != "" {
		company.SourceType = result.Source
	}
	if result.SourceURL != "" {
		company.SourceURL = result.SourceURL
	}
	company.SourceFetchedAt = &now
	if companyfetch.HasBasicInfoFootprint(company.Description, company.WebsiteURL, company.Location) {
		company.InfoFetchedAt = &now
	}
	if result.ModelUsed != "" {
		company.LastModelUsed = result.ModelUsed
	}
	if result.Confidence != "" {
		company.LastFetchConfidence = result.Confidence
	}

	if err := f.repo.Update(company); err != nil {
		return nil, fmt.Errorf("failed to update company: %w", err)
	}
	return companyInfoFromModel(company), nil
}

// ProvisionByName は DB に無い企業を取得して登録する（#1124）。
//
// 従来、履歴書レビューで未登録の企業名が来ると Web検索で実在確認だけを行い、
// **結果を捨てていた**。1コールあたり約3万入力トークン払って真偽値しか得られず、
// 企業情報が無いのでレビューは一般論になり、次に同じ企業名が来ればまた払っていた。
//
// ここでは同じ取得結果を companies に登録する。以後その企業は
// 履歴書レビュー・企業検索・マッチング・面接ヒントのすべてから
// 追加コストなしで参照できる（既存企業と同じ扱いになる）。
//
// 取得は gBizinfo（無料）を先に試すが、**AI 検索は事実上必ず走る**。
// gBizinfo は法人登記由来のため文化・働き方・主要事業を返さず、
// enrichGapsWithAI の needsAI がそれらの空欄で必ず真になるため。
// 「gBiz で済めば無料」ではなく「gBiz は AI の入力を良くするだけ」と考えること。
//
// 検索予算（SearchBudget）は SearchLiteJSON の入口で効く
// （main.go で SetSearchBudget 済み。未注入なら何も止まらない）。
//
// 十分な情報が得られなければ登録せずエラーを返す（推測で企業を作らない）。
func (f *CompanyInfoFetcher) ProvisionByName(ctx context.Context, companyName string) (*models.Company, error) {
	name := strings.TrimSpace(companyName)
	if name == "" {
		return nil, fmt.Errorf("company name is empty")
	}

	key := normalizeCompanyKey(name)
	// 直近で取得に失敗した名前は再試行しない（#1124）。
	// 打ち間違えを繰り返し投げられたときに毎回 Web検索を払わないため。
	if f.recentlyFailedProvision(key) {
		return nil, fmt.Errorf("企業情報を取得できませんでした（直近の試行が失敗）: %s", name)
	}

	// 同名の同時リクエストで二重登録しないよう singleflight を通す
	run := func() (any, error) {
		// 直前に別リクエストが登録している可能性があるので再確認する
		if existing, err := f.repo.FindByName(name); err == nil && existing != nil {
			return existing, nil
		}

		result, err := f.Acquire(ctx, name, "")
		if err != nil {
			// 取得エラーは記録しない。
			//
			// タイムアウト・ユーザー離脱・OpenAI の 5xx・空応答・JSONデコード失敗は
			// いずれも「その企業が存在しない」証拠ではなく、こちら側やプロバイダの都合。
			// 記録すると実在する企業が1時間ブロックされ、しかも DB に無いので
			// 企業検索からも選べず行き止まりになる。
			//
			// 代償として、検索が走った後で失敗するケース（JSONデコード失敗など）は
			// 再送のたびに課金される。予算超過は ErrSearchBudgetExceeded が
			// 検索の手前で返るので課金されず、ここでは問題にならない。
			// 行き止まりを作るより、その分を払うほうがましと判断している。
			return nil, err
		}
		if result == nil || !companyInfoIsSubstantive(result) {
			// 取得は成功したのに中身が無い = 打ち間違いなど「実在しない企業名」の signal。
			// ネガティブキャッシュはこの場合だけ効かせる。
			f.markProvisionFailed(key)
			return nil, fmt.Errorf("企業情報を取得できませんでした: %s", name)
		}

		now := time.Now()
		company := &models.Company{
			Name:            name,
			SourceType:      result.Source,
			SourceURL:       result.SourceURL,
			SourceFetchedAt: &now,
			InfoFetchedAt:   &now,
			// 取得直後は暫定。人手または後続の取得で確定させる
			IsProvisional:       true,
			DataStatus:          "draft",
			IsActive:            true,
			LastModelUsed:       result.ModelUsed,
			LastFetchConfidence: result.Confidence,
		}
		applyCompanyInfoResult(company, result)
		if strings.TrimSpace(company.Name) == "" {
			company.Name = name
		}
		if err := f.repo.Create(company); err != nil {
			return nil, fmt.Errorf("failed to create company: %w", err)
		}
		return company, nil
	}

	var v any
	var err error
	if f.flight != nil {
		v, err = f.flight.Do("provision", key, run)
	} else {
		v, err = run()
	}
	if err != nil {
		return nil, err
	}
	company, _ := v.(*models.Company)
	if company == nil {
		return nil, fmt.Errorf("company provision returned nil: %s", name)
	}
	return company, nil
}

// companyInfoIsSubstantive は登録に足る中身があるかを判定する。
//
// 事業内容（Description か MainBusiness）を必須にする。業種や公式URLだけでは
// 「打ち間違えた名前 ＋ 情報通信業」のレコードが学生の企業検索に載ってしまう。
// 企業briefの中核も事業内容なので、これが無い企業を登録しても
// レビューは一般論になり、登録する意味がない。
func companyInfoIsSubstantive(result *CompanyInfoResult) bool {
	return strings.TrimSpace(result.Description) != "" ||
		strings.TrimSpace(result.MainBusiness) != ""
}

// provisionFailureTTL は取得に失敗した企業名を再試行しない期間（#1124）。
//
// これが無いと、打ち間違えた企業名を投げるたびに gBizinfo + Web検索（最大3回）が
// フルで走る。singleflight は同時実行しかまとめないので、間隔を空けた再送は素通りする。
const provisionFailureTTL = time.Hour

// markProvisionFailed / recentlyFailedProvision は失敗した企業名を一定時間おぼえる。
func (f *CompanyInfoFetcher) markProvisionFailed(key string) {
	f.provisionMu.Lock()
	defer f.provisionMu.Unlock()
	if f.provisionFailures == nil {
		f.provisionFailures = map[string]time.Time{}
	}
	// 有効期限切れをここで掃除する。件数が増え続けないようにするだけなので
	// ponytail: 専用の purge ループは置かない
	now := time.Now()
	for k, at := range f.provisionFailures {
		if now.Sub(at) > provisionFailureTTL {
			delete(f.provisionFailures, k)
		}
	}
	f.provisionFailures[key] = now
}

func (f *CompanyInfoFetcher) recentlyFailedProvision(key string) bool {
	f.provisionMu.RLock()
	defer f.provisionMu.RUnlock()
	at, ok := f.provisionFailures[key]
	return ok && time.Since(at) <= provisionFailureTTL
}

// Acquire は企業名から基本情報をプレビュー取得する（DB 非更新）。
func (f *CompanyInfoFetcher) Acquire(ctx context.Context, companyName, websiteURL string) (*CompanyInfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	if f.gbiz != nil {
		if result, err := f.acquireFromGBizName(ctx, companyName); err == nil && gbizResultUseful(result) {
			return f.enrichGapsWithAI(ctx, companyName, websiteURL, result)
		}
	}

	return f.acquireViaAISearch(ctx, companyName, websiteURL)
}

func (f *CompanyInfoFetcher) acquireForCompany(ctx context.Context, company *models.Company) (*CompanyInfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if f.gbiz != nil {
		if result, err := f.acquireFromGBizCompany(ctx, company); err == nil && gbizResultUseful(result) {
			return f.enrichGapsWithAI(ctx, company.Name, company.WebsiteURL, result)
		}
	}

	return f.acquireViaAISearch(ctx, company.Name, company.WebsiteURL)
}

func (f *CompanyInfoFetcher) acquireFromGBizCompany(ctx context.Context, company *models.Company) (*CompanyInfoResult, error) {
	if strings.TrimSpace(company.CorporateNumber) == "" {
		hits, err := f.gbiz.SearchByName(ctx, company.Name)
		if err != nil || len(hits) == 0 {
			return nil, fmt.Errorf("gbizinfo: corporate number not found for %s", company.Name)
		}
		company.CorporateNumber = hits[0].CorporateNumber
		if company.WebsiteURL == "" && hits[0].CompanyURL != "" {
			company.WebsiteURL = hits[0].CompanyURL
		}
		if company.Location == "" && hits[0].Location != "" {
			company.Location = hits[0].Location
		}
		if company.EmployeeCount == 0 && hits[0].EmployeeNumber > 0 {
			company.EmployeeCount = hits[0].EmployeeNumber
			company.EmployeeCountBasis = models.EmployeeCountBasisStandalone
		}
		_ = f.repo.Update(company)
	}

	if _, err := f.gbiz.SyncCompany(ctx, company.ID); err != nil {
		return nil, err
	}
	updated, err := f.repo.FindByID(company.ID)
	if err != nil {
		return nil, err
	}
	*company = *updated

	result := companyInfoFromModel(company)
	result.Source = companyfetch.SourceGBiz
	result.Confidence = companyfetch.ConfidenceHigh
	result.ModelUsed = "gbizinfo"
	return result, nil
}

func (f *CompanyInfoFetcher) acquireFromGBizName(ctx context.Context, companyName string) (*CompanyInfoResult, error) {
	hits, err := f.gbiz.SearchByName(ctx, companyName)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("gbizinfo: no match for %s", companyName)
	}
	hit := hits[0]
	basis := ""
	if hit.EmployeeNumber > 0 {
		basis = models.EmployeeCountBasisStandalone
	}
	return &CompanyInfoResult{
		Location:           hit.Location,
		WebsiteURL:         hit.CompanyURL,
		EmployeeCount:      hit.EmployeeNumber,
		EmployeeCountBasis: basis,
		Source:             companyfetch.SourceGBiz,
		Confidence:         companyfetch.ConfidenceHigh,
		ModelUsed:          "gbizinfo",
	}, nil
}

func gbizResultUseful(r *CompanyInfoResult) bool {
	if r == nil {
		return false
	}
	return r.Location != "" || r.WebsiteURL != "" || r.EmployeeCount > 0 || r.FoundedYear > 0 || r.Description != ""
}

// enrichGapsWithAI は gBiz で埋まった結果の空欄だけ、安価 AI Search で補完する。
func (f *CompanyInfoFetcher) enrichGapsWithAI(ctx context.Context, companyName, websiteURL string, base *CompanyInfoResult) (*CompanyInfoResult, error) {
	needsAI := base.Description == "" || base.MainBusiness == "" || base.Industry == "" ||
		base.Culture == "" || base.WorkStyle == "" || base.WebsiteURL == ""
	if !needsAI {
		return base, nil
	}
	ai, err := f.acquireViaAISearch(ctx, companyName, firstNonEmpty(websiteURL, base.WebsiteURL))
	if err != nil {
		// Search モデル廃止などで AI が落ちても、gBiz の法人データは返す。
		// ここで error にするとバッチが「失敗 N」になり、保存済み gBiz も無かったことになる。
		log.Printf("ai gap enrich failed company=%s: %v (keeping gbiz)", companyName, err)
		if base.ModelUsed == "" {
			base.ModelUsed = "gbizinfo"
		}
		base.ModelUsed += "+ai_enrich_failed"
		return base, nil
	}
	mergeCompanyInfoGaps(base, ai)
	if ai.ModelUsed != "" {
		base.ModelUsed = "gbizinfo+" + ai.ModelUsed
	}
	base.Source = companyfetch.SourceGBiz + "+" + companyfetch.SourceWebSearch
	base.Confidence = companyfetch.ConfidenceMedium
	return base, nil
}

// acquireViaAISearch は安価な mini-search → Parse で事実のみ取得する（deep search なし）。
func (f *CompanyInfoFetcher) acquireViaAISearch(ctx context.Context, companyName, websiteURL string) (*CompanyInfoResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	systemPrompt := `あなたは企業情報の構造化アシスタントです。検索結果に明示された事実のみをJSON化してください。検索結果に無い項目は空文字または0。モデルの事前知識や推測で埋めてはいけません。従業員数は連結と単体を混ぜず、employee_count_basis に定義を必ず入れる。`
	siteHint := ""
	if websiteURL != "" {
		siteHint = fmt.Sprintf("（参考公式URL: %s）", websiteURL)
	}
	searchPrompt := fmt.Sprintf(
		`日本の企業「%s」%sについて、公開情報から確認できる事実だけを調べてください。対象: 企業概要・業種・本社所在地・公式サイトURL・設立年・従業員数・主要事業・企業文化・勤務スタイル・技術スタック・福利厚生。従業員数は直近有価証券報告書の連結を優先し、連結なら連結、単体しか無ければ単体と明記してください。各事実の根拠URLを含めてください。不明な項目は推測せず「不明」と書いてください。非上場企業も含め、公式サイト・採用ページ・登記情報などから確認できる範囲のみ。`,
		companyName, siteHint,
	)
	parseUser := fmt.Sprintf(
		"企業名「%s」について、検索結果の事実のみに基づき次のJSON形式で回答してください。検索結果に無い項目は空文字または0。推測禁止。\n%s",
		companyName, companyInfoJSONSchema,
	)
	raw, modelsUsed, err := f.llm.SearchLiteThenParse(ctx, searchPrompt, systemPrompt, parseUser, 600)
	if err != nil {
		return nil, fmt.Errorf("企業情報のAI取得失敗: %w", err)
	}
	result, err := parseCompanyInfoResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceWebSearch
	result.SourceURL = strings.TrimSpace(websiteURL)
	result.ModelUsed = modelsUsed
	result.Confidence = companyfetch.ConfidenceMedium
	return result, nil
}

func mergeCompanyInfoGaps(base, ai *CompanyInfoResult) {
	if base.Description == "" {
		base.Description = ai.Description
	}
	if base.Industry == "" {
		base.Industry = ai.Industry
	}
	if base.Location == "" {
		base.Location = ai.Location
	}
	if base.WebsiteURL == "" {
		base.WebsiteURL = ai.WebsiteURL
	}
	if base.FoundedYear == 0 {
		base.FoundedYear = ai.FoundedYear
	}
	if ai.EmployeeCount > 0 {
		base.EmployeeCount = ai.EmployeeCount
		base.EmployeeCountBasis = firstNonEmpty(models.NormalizeEmployeeCountBasis(ai.EmployeeCountBasis), models.EmployeeCountBasisConsolidated)
	} else if base.EmployeeCount > 0 && base.EmployeeCountBasis == "" {
		base.EmployeeCountBasis = models.EmployeeCountBasisStandalone
	}
	if base.MainBusiness == "" {
		base.MainBusiness = ai.MainBusiness
	}
	if base.Culture == "" {
		base.Culture = ai.Culture
	}
	if base.WorkStyle == "" {
		base.WorkStyle = ai.WorkStyle
	}
	if base.TechStack == "" {
		base.TechStack = ai.TechStack
	}
	if base.WelfareDetails == "" {
		base.WelfareDetails = ai.WelfareDetails
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// acquireCheapExtract は gBiz 事実テキストからの安価 Extract（Search なし）。後方互換・テスト用。
func (f *CompanyInfoFetcher) acquireCheapExtract(ctx context.Context, companyName, websiteURL, factsText string) (*CompanyInfoResult, error) {
	if f.llm == nil || f.llm.Client == nil {
		return nil, fmt.Errorf("openai client is nil")
	}

	systemPrompt := `あなたは企業情報の構造化アシスタントです。与えられた事実テキストに書かれている内容のみをJSON化してください。テキストに無い項目は空文字または0。推測禁止。`
	userPrompt := fmt.Sprintf(
		"企業名「%s」（参考URL: %s）について、次の事実テキストのみを根拠にJSON化してください。\n%s\n\n事実テキスト:\n%s",
		companyName, websiteURL, companyInfoJSONSchema, strings.TrimSpace(factsText),
	)
	if strings.TrimSpace(factsText) == "" {
		return f.acquireViaAISearch(ctx, companyName, websiteURL)
	}

	raw, model, err := f.llm.ExtractJSON(ctx, systemPrompt, userPrompt, 600)
	if err != nil {
		return nil, fmt.Errorf("企業情報の取得失敗: %w", err)
	}
	result, err := parseCompanyInfoResult(raw)
	if err != nil {
		return nil, err
	}
	result.Source = companyfetch.SourceLLMExtract
	result.ModelUsed = model
	result.Confidence = companyfetch.ConfidenceMedium
	return result, nil
}

func parseCompanyInfoResult(text string) (*CompanyInfoResult, error) {
	obj, err := companyfetch.ExtractJSONObject(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ai response: %w", err)
	}
	var result CompanyInfoResult
	if err := json.Unmarshal([]byte(obj), &result); err != nil {
		return nil, fmt.Errorf("failed to parse company info json: %w", err)
	}
	result.EmployeeCountBasis = models.NormalizeEmployeeCountBasis(result.EmployeeCountBasis)
	if result.EmployeeCount > 0 && result.EmployeeCountBasis == "" {
		result.EmployeeCountBasis = models.EmployeeCountBasisConsolidated
	}
	return &result, nil
}

func companyInfoFromModel(company *models.Company) *CompanyInfoResult {
	return &CompanyInfoResult{
		Description:        company.Description,
		Industry:           company.Industry,
		Location:           company.Location,
		WebsiteURL:         company.WebsiteURL,
		FoundedYear:        company.FoundedYear,
		EmployeeCount:      company.EmployeeCount,
		EmployeeCountBasis: company.EmployeeCountBasis,
		MainBusiness:       company.MainBusiness,
		Culture:            company.Culture,
		WorkStyle:          company.WorkStyle,
		TechStack:          company.TechStack,
		WelfareDetails:     company.WelfareDetails,
		Source:             company.SourceType,
		SourceURL:          company.SourceURL,
		ModelUsed:          company.LastModelUsed,
		Confidence:         company.LastFetchConfidence,
	}
}

func markInfoCacheSkip(r *CompanyInfoResult, reason string, budgetExceeded bool) *CompanyInfoResult {
	if r == nil {
		r = &CompanyInfoResult{}
	}
	r.FromCache = true
	r.SkipReason = reason
	r.BudgetExceeded = budgetExceeded
	return r
}

func applyCompanyInfoResult(company *models.Company, result *CompanyInfoResult) {
	if result.Description != "" {
		company.Description = result.Description
	}
	if result.Industry != "" {
		company.Industry = result.Industry
	}
	if result.Location != "" {
		company.Location = result.Location
	}
	if result.WebsiteURL != "" {
		company.WebsiteURL = result.WebsiteURL
	}
	if result.FoundedYear > 0 {
		company.FoundedYear = result.FoundedYear
	}
	if result.EmployeeCount > 0 {
		company.EmployeeCount = result.EmployeeCount
		if basis := models.NormalizeEmployeeCountBasis(result.EmployeeCountBasis); basis != "" {
			company.EmployeeCountBasis = basis
		}
	}
	if result.MainBusiness != "" {
		company.MainBusiness = result.MainBusiness
	}
	if result.Culture != "" {
		company.Culture = result.Culture
	}
	if result.WorkStyle != "" {
		company.WorkStyle = result.WorkStyle
	}
	if result.TechStack != "" {
		company.TechStack = result.TechStack
	}
	if result.WelfareDetails != "" {
		company.WelfareDetails = result.WelfareDetails
	}
}

func buildCompanyFactsText(company *models.Company) string {
	var b strings.Builder
	fmt.Fprintf(&b, "正式名称: %s\n", company.Name)
	if company.CorporateNumber != "" {
		fmt.Fprintf(&b, "法人番号: %s\n", company.CorporateNumber)
	}
	if company.Location != "" {
		fmt.Fprintf(&b, "所在地: %s\n", company.Location)
	}
	if company.WebsiteURL != "" {
		fmt.Fprintf(&b, "公式URL: %s\n", company.WebsiteURL)
	}
	if company.FoundedYear > 0 {
		fmt.Fprintf(&b, "設立年: %d\n", company.FoundedYear)
	}
	if company.EmployeeCount > 0 {
		fmt.Fprintf(&b, "従業員数: %s\n", models.FormatEmployeeCount(company.EmployeeCount, company.EmployeeCountBasis))
	}
	if company.Industry != "" {
		fmt.Fprintf(&b, "業種: %s\n", company.Industry)
	}
	if company.Description != "" {
		fmt.Fprintf(&b, "概要: %s\n", company.Description)
	}
	if company.MainBusiness != "" {
		fmt.Fprintf(&b, "主要事業: %s\n", company.MainBusiness)
	}
	return b.String()
}
