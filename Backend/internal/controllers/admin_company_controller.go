package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminCompanyController struct {
	repo             repository.CompanyRepository
	audit            ifaces.AuditLogService
	gbiz             *services.GBizInfoService
	openaiClient     *openai.Client
	infoFetcher      *services.CompanyInfoFetcher
	relationsFetcher *services.CompanyRelationsFetcher
	jobFetcher       *services.JobFetchService
	techFetcher      *services.TechStackFetcher
	catalogWarm      *services.CatalogWarmService
	missingBatch     *services.CompanyMissingBatchService
}

func NewAdminCompanyController(repo repository.CompanyRepository, audit ifaces.AuditLogService, gbiz *services.GBizInfoService, openaiClient ...*openai.Client) *AdminCompanyController {
	ctrl := &AdminCompanyController{repo: repo, audit: audit, gbiz: gbiz}
	if len(openaiClient) > 0 {
		ctrl.openaiClient = openaiClient[0]
		ctrl.infoFetcher = services.NewCompanyInfoFetcher(repo, openaiClient[0], gbiz)
		ctrl.jobFetcher = services.NewJobFetchService(repo, openaiClient[0])
		ctrl.techFetcher = services.NewTechStackFetcher(repo, openaiClient[0])
		ctrl.catalogWarm = services.NewCatalogWarmService(repo, ctrl.infoFetcher, ctrl.jobFetcher)
		ctrl.missingBatch = services.NewCompanyMissingBatchService(
			repo, ctrl.infoFetcher, ctrl.jobFetcher, ctrl.techFetcher, nil,
		)
	}
	return ctrl
}

// SetRelationsFetcher は企業関係・市場情報取得サービスを注入する（#633 Phase 2）。
func (c *AdminCompanyController) SetRelationsFetcher(fetcher *services.CompanyRelationsFetcher) {
	if c != nil {
		c.relationsFetcher = fetcher
		if c.missingBatch != nil || c.infoFetcher != nil {
			c.missingBatch = services.NewCompanyMissingBatchService(
				c.repo, c.infoFetcher, c.jobFetcher, c.techFetcher, fetcher,
			)
		}
	}
}

// SetCompanySearchGuards は FirstTouch Search の予算・singleflight を注入する（#587）。
func (c *AdminCompanyController) SetCompanySearchGuards(budget companyfetch.SearchBudget, flight *services.CompanySearchFlight) {
	if c == nil {
		return
	}
	if c.infoFetcher != nil {
		c.infoFetcher.SetSearchBudget(budget)
		c.infoFetcher.SetSearchFlight(flight)
	}
	if c.jobFetcher != nil {
		c.jobFetcher.SetSearchBudget(budget)
		c.jobFetcher.SetSearchFlight(flight)
	}
	if c.techFetcher != nil {
		c.techFetcher.SetSearchBudget(budget)
		c.techFetcher.SetSearchFlight(flight)
	}
	if c.relationsFetcher != nil {
		c.relationsFetcher.SetSearchBudget(budget)
		c.relationsFetcher.SetSearchFlight(flight)
	}
}

// List GET /api/admin/companies
func (c *AdminCompanyController) List(ctx echo.Context) error {
	const maxListLimit = 200
	limit := 50
	offset := 0
	if v, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && v > 0 {
		limit = min(v, maxListLimit)
	}
	if v, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && v >= 0 {
		offset = v
	}
	name := strings.TrimSpace(ctx.QueryParam("name"))
	status := strings.TrimSpace(ctx.QueryParam("status"))
	industry := strings.TrimSpace(ctx.QueryParam("industry"))
	readiness := strings.TrimSpace(ctx.QueryParam("readiness"))
	orderBy := strings.TrimSpace(ctx.QueryParam("order"))
	// 企業カタログは共有のため school_id は「承認済みだけ見る」任意の絞り込み(閲覧制限ではない)。
	var schoolID *uint
	if raw := ctx.QueryParam("school_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			v := uint(id)
			schoolID = &v
		}
	}
	companies, total, err := c.repo.ListActiveFiltered(limit, offset, name, status, industry, readiness, orderBy, schoolID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch companies")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"companies": companies,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
		"name":      name,
		"status":    status,
		"industry":  industry,
		"readiness": readiness,
		"order":     orderBy,
	})
}

// Industries GET /api/admin/companies/industries
// アクティブ企業に付いている業界名の一覧を返す（絞り込み用）。
func (c *AdminCompanyController) Industries(ctx echo.Context) error {
	industries, err := c.repo.ListActiveIndustries()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch industries")
	}
	if industries == nil {
		industries = []string{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"industries": industries})
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
	// AI プレビュー経由で作成した場合は取得メタを残し、公開後も再取得判断できるようにする
	if strings.TrimSpace(payload.LastModelUsed) != "" && payload.InfoFetchedAt == nil {
		now := time.Now()
		payload.InfoFetchedAt = &now
	}
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
