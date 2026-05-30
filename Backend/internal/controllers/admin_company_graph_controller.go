package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/scraper"
	ifaces "Backend/internal/services/interfaces"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AdminCompanyGraphController exposes the multi-source scraping pipeline via HTTP.
type AdminCompanyGraphController struct {
	pipeline     *scraper.Pipeline
	companyRepo  repository.CompanyRepository
	relationRepo repository.CompanyRelationRepository
	audit        ifaces.AuditLogService
	openaiClient *openai.Client
}

func NewAdminCompanyGraphController(pipeline *scraper.Pipeline, companyRepo repository.CompanyRepository, relationRepo repository.CompanyRelationRepository, audit ifaces.AuditLogService, openaiClient *openai.Client) *AdminCompanyGraphController {
	return &AdminCompanyGraphController{pipeline: pipeline, companyRepo: companyRepo, relationRepo: relationRepo, audit: audit, openaiClient: openaiClient}
}

// TargetYear handles GET /api/admin/company-graph/target-year
func (c *AdminCompanyGraphController) TargetYear(ctx echo.Context) error {
	override := 0
	if v := ctx.QueryParam("year"); v != "" {
		override, _ = strconv.Atoi(v)
	}
	y := scraper.ResolveYear(override)
	return ctx.JSON(http.StatusOK, map[string]int{"target_year": y})
}

// Crawl handles POST /api/admin/company-graph/crawl
func (c *AdminCompanyGraphController) Crawl(ctx echo.Context) error {
	var req struct {
		Sites     []string `json:"sites"`
		Query     string   `json:"query"`
		Pages     int      `json:"pages"`
		Year      int      `json:"year"`
		Threshold float64  `json:"threshold"`
	}
	req.Sites = []string{"rikunabi", "career_tasu"}
	req.Query = "IT"
	req.Pages = 2
	req.Threshold = config.CompanyGraphThreshold()

	ctx.Bind(&req)

	// company-graph コンテナ経由でクロール（未設定の場合は埋め込みパイプラインを使用）
	var nodes map[string]*scraper.CompanyNode
	var logs string
	var targetYear int

	companyGraphURL := os.Getenv("COMPANY_GRAPH_URL")
	if companyGraphURL != "" {
		n, l, y, err := c.crawlViaService(ctx, companyGraphURL, req.Sites, req.Query, req.Pages, req.Year, req.Threshold)
		if err != nil {
			return ctx.JSON(http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
		}
		nodes, logs, targetYear = n, l, y
	} else {
		if c.pipeline == nil {
			return echoInternalError(errors.New("pipeline not configured"))
		}
		p := *c.pipeline
		if req.Threshold > 0 {
			p.Threshold = req.Threshold
		}
		result, err := p.Run(ctx.Request().Context(), scraper.RunRequest{
			Sites:    req.Sites,
			Query:    req.Query,
			MaxPages: req.Pages,
			Year:     req.Year,
		})
		if err != nil && (result == nil || len(result.Nodes) == 0) {
			l := ""
			if result != nil {
				l = strings.Join(result.Logs, "\n")
			}
			return ctx.JSON(http.StatusUnprocessableEntity, map[string]any{"ok": false, "error": err.Error(), "logs": l})
		}
		if result != nil {
			nodes = result.Nodes
			logs = strings.Join(result.Logs, "\n")
			targetYear = result.TargetYear
		}
	}

	// DB 保存
	saved, skipped := c.upsertNodes(nodes)

	// スクレイピングで取得した関連会社・取引先を company_relations に同期
	relSynced := c.syncRelationsFromNodes(nodes)

	adminEmail := ctx.Request().Header.Get("X-Admin-Email")
	if adminEmail != "" && c.audit != nil {
		c.audit.Record(adminEmail, "company_graph_crawl", "pipeline", 0, map[string]any{
			"sites": req.Sites,
			"query": req.Query,
			"nodes": len(nodes),
			"saved": saved,
		})
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"ok":               true,
		"logs":             logs,
		"nodes":            len(nodes),
		"saved":            saved,
		"skipped":          skipped,
		"target_year":      targetYear,
		"relations_synced": relSynced,
	})
}

// crawlViaService は company-graph コンテナを呼び出してノードデータを取得する。
func (c *AdminCompanyGraphController) crawlViaService(
	ctx echo.Context,
	baseURL string,
	sites []string, query string, pages, year int, threshold float64,
) (map[string]*scraper.CompanyNode, string, int, error) {

	payload, _ := json.Marshal(map[string]any{
		"sites":     sites,
		"query":     query,
		"pages":     pages,
		"year":      year,
		"threshold": threshold,
	})

	req, err := http.NewRequestWithContext(ctx.Request().Context(), http.MethodPost, baseURL+"/crawl", bytes.NewReader(payload))
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 310 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK         bool                            `json:"ok"`
		Error      string                          `json:"error"`
		Logs       string                          `json:"logs"`
		TargetYear int                             `json:"target_year"`
		Nodes      map[string]*scraper.CompanyNode `json:"nodes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", 0, err
	}
	if !result.OK {
		return nil, result.Logs, result.TargetYear, errors.New(result.Error)
	}
	return result.Nodes, result.Logs, result.TargetYear, nil
}

var reRelationSep = regexp.MustCompile(`[,、，\r\n]+`)
var reRelationNoise = regexp.MustCompile(`(?:など|等|・$|その他.*$)`)

// parseCompanyNames はスクレイピングで得た関係テキストを企業名リストに分解する。
func parseCompanyNames(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := reRelationSep.Split(text, -1)
	var names []string
	for _, p := range parts {
		p = reRelationNoise.ReplaceAllString(strings.TrimSpace(p), "")
		p = strings.TrimSpace(p)
		if len([]rune(p)) >= 2 {
			names = append(names, p)
		}
	}
	return names
}

// syncRelationsFromNodes はスクレイピングノードの関連会社・取引先テキストを解析し、
// company_relations テーブルへ Upsert する。
// 戻り値: 登録成功した関係レコード件数
func (c *AdminCompanyGraphController) syncRelationsFromNodes(nodes map[string]*scraper.CompanyNode) int {
	if c.relationRepo == nil || c.companyRepo == nil {
		return 0
	}
	now := time.Now()
	synced := 0

	for _, node := range nodes {
		if node == nil {
			continue
		}
		// 法人番号で元企業を特定（UNKNOWN_プレフィックスは名寄せ未済のためスキップ）
		if strings.HasPrefix(node.CorporateNumber, "UNKNOWN_") {
			continue
		}
		fromCompany, err := c.companyRepo.FindByCorporateNumber(node.CorporateNumber)
		if err != nil || fromCompany == nil {
			continue
		}

		type relEntry struct {
			text         string
			relationType string
		}
		entries := []relEntry{
			{node.RelatedCompaniesText, "capital_affiliate"},
			{node.BusinessPartnersText, "business_partner"},
		}

		for _, entry := range entries {
			names := parseCompanyNames(entry.text)
			for _, name := range names {
				// 既存企業を名前で検索、なければプロビジョナル企業として登録
				toCompany, err := c.companyRepo.FindByName(name)
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				if toCompany == nil {
					toCompany = &models.Company{
						Name:            name,
						SourceType:      "scraping",
						SourceFetchedAt: &now,
						IsProvisional:   true,
						DataStatus:      "draft",
					}
					if err := c.companyRepo.Create(toCompany); err != nil {
						continue
					}
				}
				desc := fmt.Sprintf("scraping:%s", node.OfficialName)
				if err := c.relationRepo.UpsertBusinessRelation(fromCompany.ID, toCompany.ID, entry.relationType, desc); err != nil {
					continue
				}
				synced++
			}
		}
	}
	return synced
}

// upsertNodes は CompanyNode を companies テーブルへ upsert する。
// 法人番号が既存のレコードと一致する場合は更新、なければ新規作成。
// 戻り値: (saved件数, skipped件数)
func (c *AdminCompanyGraphController) upsertNodes(nodes map[string]*scraper.CompanyNode) (int, int) {
	if c.companyRepo == nil {
		return 0, len(nodes)
	}
	now := time.Now()
	saved, skipped := 0, 0

	for _, node := range nodes {
		if node == nil || strings.HasPrefix(node.CorporateNumber, "UNKNOWN_") {
			skipped++
			continue
		}

		existing, err := c.companyRepo.FindByCorporateNumber(node.CorporateNumber)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			skipped++
			continue
		}

		sourceURL := ""
		if len(node.SourceURLs) > 0 {
			sourceURL = node.SourceURLs[0]
		}

		if existing == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			company := &models.Company{
				Name:             node.OfficialName,
				CorporateNumber:  node.CorporateNumber,
				Industry:         node.BusinessCategory,
				Location:         node.Address,
				WebsiteURL:       node.Website,
				SourceType:       "job_site",
				SourceURL:        sourceURL,
				SourceFetchedAt:  &now,
				IsProvisional:    node.NeedsReview,
				DataStatus:       "draft",
				GBizLastSyncedAt: &now,
				GBizSyncStatus:   "success",
			}
			if err := c.companyRepo.Create(company); err != nil {
				skipped++
				continue
			}
		} else {
			if node.BusinessCategory != "" {
				existing.Industry = node.BusinessCategory
			}
			if node.Address != "" {
				existing.Location = node.Address
			}
			if node.Website != "" {
				existing.WebsiteURL = node.Website
			}
			if sourceURL != "" {
				existing.SourceURL = sourceURL
			}
			existing.IsProvisional = node.NeedsReview
			existing.SourceFetchedAt = &now
			existing.GBizLastSyncedAt = &now
			existing.GBizSyncStatus = "success"
			if err := c.companyRepo.Update(existing); err != nil {
				skipped++
				continue
			}
		}
		saved++
	}
	return saved, skipped
}

// llmExtractedRelations はOpenAI Web Searchで抽出した企業関係データ。
type llmExtractedRelations struct {
	Subsidiaries     []string `json:"subsidiaries"`
	Affiliates       []string `json:"affiliates"`
	BusinessPartners []string `json:"business_partners"`
}

// EnrichRelations handles POST /api/admin/company-graph/enrich-relations
// OpenAI Web Searchを使って指定企業の関連会社・取引先を取得し、DBに保存する。
func (c *AdminCompanyGraphController) EnrichRelations(ctx echo.Context) error {
	var req struct {
		CompanyID uint   `json:"company_id"`
		URL       string `json:"url"`
	}
	if err := ctx.Bind(&req); err != nil || req.CompanyID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id is required")
	}

	company, err := c.companyRepo.FindByID(req.CompanyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	extracted, err := c.fetchRelationsWithLLM(ctx.Request().Context(), company.Name, req.URL)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	now := time.Now()
	saved := 0

	type relEntry struct {
		names        []string
		relationType string
	}
	entries := []relEntry{
		{extracted.Subsidiaries, "capital_subsidiary"},
		{extracted.Affiliates, "capital_affiliate"},
		{extracted.BusinessPartners, "business_partner"},
	}

	for _, entry := range entries {
		for _, name := range entry.names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			toCompany, findErr := c.companyRepo.FindByName(name)
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				continue
			}
			if toCompany == nil {
				toCompany = &models.Company{
					Name:            name,
					SourceType:      "llm_web_search",
					SourceFetchedAt: &now,
					IsProvisional:   true,
					DataStatus:      "draft",
				}
				if createErr := c.companyRepo.Create(toCompany); createErr != nil {
					continue
				}
			}
			desc := fmt.Sprintf("llm_web_search:%s", company.Name)
			if upsertErr := c.relationRepo.UpsertBusinessRelation(company.ID, toCompany.ID, entry.relationType, desc); upsertErr != nil {
				continue
			}
			saved++
		}
	}

	adminEmail := ctx.Request().Header.Get("X-Admin-Email")
	if adminEmail != "" && c.audit != nil {
		c.audit.Record(adminEmail, "company_graph_enrich_relations", "company", company.ID, map[string]any{
			"company": company.Name,
			"saved":   saved,
		})
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"ok":        true,
		"company":   company.Name,
		"saved":     saved,
		"extracted": extracted,
	})
}

// fetchRelationsWithLLM はOpenAI Web Searchで企業の関連会社・取引先を抽出する。
// urlを指定した場合はそのページを優先的に参照する。
func (c *AdminCompanyGraphController) fetchRelationsWithLLM(ctx context.Context, companyName, url string) (*llmExtractedRelations, error) {
	if c.openaiClient == nil {
		return nil, errors.New("openai client not configured")
	}

	var prompt string
	if url != "" {
		prompt = fmt.Sprintf(
			`次のURL「%s」を参照して、「%s」の企業関係情報を調べてください。子会社・グループ会社・資本提携先・主要取引先を抽出し、実在する企業名のみを以下のJSON形式のみで返してください。余分な説明は不要です。
{"subsidiaries":["子会社・グループ会社名"],"affiliates":["資本提携・関連会社名"],"business_partners":["主要取引先名"]}`,
			url, companyName,
		)
	} else {
		prompt = fmt.Sprintf(
			`「%s」の企業関係情報をウェブで検索してください。子会社・グループ会社・資本提携先・主要取引先を調べて、実在する企業名のみを以下のJSON形式のみで返してください。余分な説明は不要です。
{"subsidiaries":["子会社・グループ会社名"],"affiliates":["資本提携・関連会社名"],"business_partners":["主要取引先名"]}`,
			companyName,
		)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	text, err := c.openaiClient.WebSearchQuery(ctxTimeout, prompt)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return &llmExtractedRelations{}, nil
	}

	var result llmExtractedRelations
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return &llmExtractedRelations{}, nil
	}
	return &result, nil
}
