package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminCompanyController struct {
	repo         repository.CompanyRepository
	audit        ifaces.AuditLogService
	gbiz         *services.GBizInfoService
	openaiClient *openai.Client
	infoFetcher  *services.CompanyInfoFetcher
	jobFetcher   *services.JobFetchService
	techFetcher  *services.TechStackFetcher
	catalogWarm  *services.CatalogWarmService
}

func NewAdminCompanyController(repo repository.CompanyRepository, audit ifaces.AuditLogService, gbiz *services.GBizInfoService, openaiClient ...*openai.Client) *AdminCompanyController {
	ctrl := &AdminCompanyController{repo: repo, audit: audit, gbiz: gbiz}
	if len(openaiClient) > 0 {
		ctrl.openaiClient = openaiClient[0]
		ctrl.infoFetcher = services.NewCompanyInfoFetcher(repo, openaiClient[0], gbiz)
		ctrl.jobFetcher = services.NewJobFetchService(repo, openaiClient[0])
		ctrl.techFetcher = services.NewTechStackFetcher(repo, openaiClient[0])
		ctrl.catalogWarm = services.NewCatalogWarmService(repo, ctrl.infoFetcher, ctrl.jobFetcher)
	}
	return ctrl
}

// List GET /api/admin/companies
func (c *AdminCompanyController) List(ctx echo.Context) error {
	limit := 50
	offset := 0
	if v, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && v >= 0 {
		offset = v
	}
	companies, err := c.repo.FindAllActive(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch companies")
	}
	total, _ := c.repo.CountActive()
	return ctx.JSON(http.StatusOK, map[string]any{
		"companies": companies,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// Create POST /api/admin/companies
func (c *AdminCompanyController) Create(ctx echo.Context) error {
	var payload models.Company
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(payload.Name) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	applyCompanyDefaults(&payload)
	if err := c.repo.Create(&payload); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.create", "company", payload.ID, map[string]any{
		"name": payload.Name,
	})
	return ctx.JSON(http.StatusOK, payload)
}

// Get GET /api/admin/companies/:id
func (c *AdminCompanyController) Get(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	return ctx.JSON(http.StatusOK, company)
}

// Update PUT /api/admin/companies/:id
func (c *AdminCompanyController) Update(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	var payload models.Company
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	if err := mergeCompany(company, &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.update", "company", company.ID, map[string]any{
		"name": company.Name,
	})
	return ctx.JSON(http.StatusOK, company)
}

// Publish PATCH /api/admin/companies/:id/publish
func (c *AdminCompanyController) Publish(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	company.DataStatus = "published"
	company.IsProvisional = false
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to publish company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.publish", "company", company.ID, map[string]any{
		"name": company.Name,
	})
	return ctx.JSON(http.StatusOK, company)
}

// Reject PATCH /api/admin/companies/:id/reject
func (c *AdminCompanyController) Reject(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}
	company.IsActive = false
	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reject company")
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.reject", "company", company.ID, map[string]any{
		"name": company.Name,
	})
	return ctx.JSON(http.StatusOK, map[string]string{"status": "rejected"})
}

// SearchGBiz GET /api/admin/companies/search-gbiz?name=xxx
func (c *AdminCompanyController) SearchGBiz(ctx echo.Context) error {
	name := strings.TrimSpace(ctx.QueryParam("name"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if c.gbiz == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gbizinfo service not configured")
	}
	results, err := c.gbiz.SearchByName(ctx.Request().Context(), name)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"results": results})
}

// SyncGBiz POST /api/admin/companies/:id/gbiz-sync
func (c *AdminCompanyController) SyncGBiz(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.gbiz == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "gbizinfo service not configured")
	}
	result, err := c.gbiz.SyncCompany(ctx.Request().Context(), uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.gbiz_sync", "company", uint(id), map[string]any{
		"status": result.Status,
	})
	return ctx.JSON(http.StatusOK, result)
}

// WebSearchCompanyInfo POST /api/admin/companies/web-search
// 企業名をもとにスクレイプ/Search→Parse で企業情報をプレビュー用に返す（DB非更新）
func (c *AdminCompanyController) WebSearchCompanyInfo(ctx echo.Context) error {
	var req struct {
		Name       string `json:"name"`
		WebsiteURL string `json:"website_url"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(req.Name) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if c.infoFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	result, err := c.infoFetcher.Acquire(ctx.Request().Context(), req.Name, req.WebsiteURL)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.web_search", "company", 0, map[string]any{
		"name":   req.Name,
		"source": result.Source,
		"model":  result.ModelUsed,
	})

	return ctx.JSON(http.StatusOK, result)
}

// FetchTechStack POST /api/admin/companies/:id/tech-stack-search
// スクレイプ/Search から技術スタックを取得してDBを更新する
func (c *AdminCompanyController) FetchTechStack(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.techFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	forceRefresh := ctx.QueryParam("force") == "true"
	result, err := c.techFetcher.FetchAndSave(ctx.Request().Context(), uint(id), forceRefresh)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_tech_stack", "company", uint(id), map[string]any{
		"force":  forceRefresh,
		"source": result.Source,
		"model":  result.ModelUsed,
	})

	return ctx.JSON(http.StatusOK, result)
}

// FetchCompanyInfo POST /api/admin/companies/:id/fetch-info
// ?force=true を付けると DB キャッシュを無視して AI で再取得する。
func (c *AdminCompanyController) FetchCompanyInfo(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.infoFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	forceRefresh := ctx.QueryParam("force") == "true"
	result, err := c.infoFetcher.FetchAndSave(ctx.Request().Context(), uint(id), forceRefresh)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_info", "company", uint(id), map[string]any{
		"industry": result.Industry,
		"force":    forceRefresh,
	})

	return ctx.JSON(http.StatusOK, result)
}

// FetchJobs POST /api/admin/companies/:id/fetch-jobs
// ?force=true を付けると DB キャッシュを無視して AI で再取得する。
func (c *AdminCompanyController) FetchJobs(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.jobFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	forceRefresh := ctx.QueryParam("force") == "true"
	positions, err := c.jobFetcher.FetchAndSaveJobs(ctx.Request().Context(), uint(id), forceRefresh)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_jobs", "company", uint(id), map[string]any{
		"count": len(positions),
		"force": forceRefresh,
	})

	return ctx.JSON(http.StatusOK, map[string]any{
		"positions": positions,
		"total":     len(positions),
	})
}

// FetchPersona POST /api/admin/companies/:id/fetch-persona
// ?force=true を付けると DB キャッシュを無視して AI で再取得する。
func (c *AdminCompanyController) FetchPersona(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.jobFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	forceRefresh := ctx.QueryParam("force") == "true"
	profile, err := c.jobFetcher.FetchAndSavePersona(ctx.Request().Context(), uint(id), forceRefresh)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_persona", "company", uint(id), map[string]any{
		"force": forceRefresh,
	})

	return ctx.JSON(http.StatusOK, profile)
}

// SeedL1Catalog POST /api/admin/companies/seed-l1
// multipart file=csv または body に CSV テキスト。未指定時は sample CSV を読む。
func (c *AdminCompanyController) SeedL1Catalog(ctx echo.Context) error {
	var reader io.Reader
	file, err := ctx.FormFile("file")
	if err == nil && file != nil {
		f, openErr := file.Open()
		if openErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, openErr.Error())
		}
		defer f.Close()
		reader = f
	} else if body := ctx.Request().Body; body != nil && ctx.Request().ContentLength > 0 &&
		strings.Contains(ctx.Request().Header.Get("Content-Type"), "text/csv") {
		reader = body
	} else {
		path := strings.TrimSpace(ctx.QueryParam("path"))
		if path == "" {
			path = "config/l1_catalog_seed.sample.csv"
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			// Backend 作業ディレクトリ差を吸収
			alt := filepath.Join("Backend", path)
			f2, err2 := os.Open(alt)
			if err2 != nil {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("seed file not found: %v", openErr))
			}
			defer f2.Close()
			reader = f2
		} else {
			defer f.Close()
			reader = f
		}
	}
	rows, err := services.ParseL1SeedCSV(reader)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	result, err := services.ImportL1Seed(c.repo, rows)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.seed_l1", "company", 0, map[string]any{
		"total":   result.Total,
		"created": result.Created,
		"updated": result.Updated,
	})
	return ctx.JSON(http.StatusOK, result)
}

// GetL1Coverage GET /api/admin/companies/l1-coverage
func (c *AdminCompanyController) GetL1Coverage(ctx echo.Context) error {
	if c.catalogWarm == nil {
		c.catalogWarm = services.NewCatalogWarmService(c.repo, c.infoFetcher, c.jobFetcher)
	}
	cov, err := c.catalogWarm.Coverage(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, cov)
}

// WarmL1Catalog POST /api/admin/companies/warm-l1
// Body: { "limit": 100, "dry_run": true, "force": false, "include_info": true, "include_persona": true }
func (c *AdminCompanyController) WarmL1Catalog(ctx echo.Context) error {
	if c.catalogWarm == nil {
		c.catalogWarm = services.NewCatalogWarmService(c.repo, c.infoFetcher, c.jobFetcher)
	}
	var opts services.L1WarmOptions
	opts.IncludeInfo = true
	opts.IncludePersona = true
	if err := ctx.Bind(&opts); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	// Bind で false が欠落した場合の既定は normalize 側でも補完する
	result, err := c.catalogWarm.WarmL1(ctx.Request().Context(), opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.warm_l1", "company", 0, map[string]any{
		"dry_run":    result.DryRun,
		"limit":      result.Limit,
		"processed":  result.Processed,
		"info_ok":    result.InfoOK,
		"persona_ok": result.PersonaOK,
		"errors":     result.Errors,
	})
	return ctx.JSON(http.StatusOK, result)
}

func applyCompanyDefaults(company *models.Company) {
	if strings.TrimSpace(company.SourceType) == "" {
		company.SourceType = "manual"
	}
	if strings.TrimSpace(company.DataStatus) == "" {
		company.DataStatus = "draft"
	}
	if company.SourceFetchedAt == nil {
		now := time.Now()
		company.SourceFetchedAt = &now
	}
	if company.IsProvisional == false && strings.TrimSpace(company.SourceURL) == "" {
		company.IsProvisional = true
	}
	if strings.TrimSpace(company.Name) != "" && company.IsProvisional == false {
		company.IsVerified = true
	}
}

func mergeCompany(existing *models.Company, payload *models.Company) error {
	if strings.TrimSpace(payload.Name) != "" {
		existing.Name = payload.Name
	}
	if strings.TrimSpace(payload.Description) != "" {
		existing.Description = payload.Description
	}
	if strings.TrimSpace(payload.Industry) != "" {
		existing.Industry = payload.Industry
	}
	if strings.TrimSpace(payload.Location) != "" {
		existing.Location = payload.Location
	}
	if payload.EmployeeCount > 0 {
		existing.EmployeeCount = payload.EmployeeCount
	}
	if payload.FoundedYear > 0 {
		existing.FoundedYear = payload.FoundedYear
	}
	if strings.TrimSpace(payload.WebsiteURL) != "" {
		existing.WebsiteURL = payload.WebsiteURL
	}
	if strings.TrimSpace(payload.LogoURL) != "" {
		existing.LogoURL = payload.LogoURL
	}
	if strings.TrimSpace(payload.CorporateNumber) != "" {
		existing.CorporateNumber = payload.CorporateNumber
	}
	if strings.TrimSpace(payload.MainBusiness) != "" {
		existing.MainBusiness = payload.MainBusiness
	}
	if strings.TrimSpace(payload.Culture) != "" {
		existing.Culture = payload.Culture
	}
	if strings.TrimSpace(payload.WorkStyle) != "" {
		existing.WorkStyle = payload.WorkStyle
	}
	if strings.TrimSpace(payload.WelfareDetails) != "" {
		existing.WelfareDetails = payload.WelfareDetails
	}
	if strings.TrimSpace(payload.TechStack) != "" {
		existing.TechStack = payload.TechStack
	}
	if strings.TrimSpace(payload.InfraStack) != "" {
		existing.InfraStack = payload.InfraStack
	}
	if strings.TrimSpace(payload.CicdTools) != "" {
		existing.CicdTools = payload.CicdTools
	}
	if strings.TrimSpace(payload.DevelopmentStyle) != "" {
		existing.DevelopmentStyle = payload.DevelopmentStyle
	}
	if strings.TrimSpace(payload.SourceType) != "" {
		existing.SourceType = payload.SourceType
	}
	if strings.TrimSpace(payload.SourceURL) != "" {
		existing.SourceURL = payload.SourceURL
	}
	if payload.SourceFetchedAt != nil {
		existing.SourceFetchedAt = payload.SourceFetchedAt
	}
	if payload.DataStatus != "" {
		if payload.DataStatus != "draft" && payload.DataStatus != "published" {
			return errors.New("data_status must be draft or published")
		}
		existing.DataStatus = payload.DataStatus
	}
	existing.IsProvisional = payload.IsProvisional
	return nil
}
