package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	ifaces "Backend/internal/services/interfaces"
	"context"
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
		if c.missingBatch != nil {
			c.missingBatch = services.NewCompanyMissingBatchService(
				c.repo, c.infoFetcher, c.jobFetcher, c.techFetcher, fetcher,
			)
		} else if c.infoFetcher != nil {
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
	limit := 50
	offset := 0
	if v, err := strconv.Atoi(ctx.QueryParam("limit")); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(ctx.QueryParam("offset")); err == nil && v >= 0 {
		offset = v
	}
	name := strings.TrimSpace(ctx.QueryParam("name"))
	status := strings.TrimSpace(ctx.QueryParam("status"))
	companies, total, err := c.repo.ListActiveFiltered(limit, offset, name, status)
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

// ConfirmCompanyInfo POST /api/admin/companies/:id/confirm-info
// プレビュー済みの構造化結果を LLM 再実行なしで DB に確定保存する。
func (c *AdminCompanyController) ConfirmCompanyInfo(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.infoFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	var req services.CompanyInfoResult
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	result, err := c.infoFetcher.ConfirmAndSave(uint(id), &req)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.confirm_info", "company", uint(id), map[string]any{
		"source":     result.Source,
		"model":      result.ModelUsed,
		"confidence": result.Confidence,
	})

	return ctx.JSON(http.StatusOK, result)
}

// WebSearchCompanyRelations POST /api/admin/companies/web-search-relations
// 企業名をもとに Search→Parse で関係・市場情報をプレビュー用に返す（DB非更新）
func (c *AdminCompanyController) WebSearchCompanyRelations(ctx echo.Context) error {
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
	if c.relationsFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	result, err := c.relationsFetcher.Acquire(ctx.Request().Context(), req.Name, req.WebsiteURL)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.web_search_relations", "company", 0, map[string]any{
		"name":   req.Name,
		"source": result.Source,
		"model":  result.ModelUsed,
	})

	return ctx.JSON(http.StatusOK, result)
}

// FetchCompanyRelations POST /api/admin/companies/:id/fetch-relations
// ?force=true を付けると DB キャッシュを無視して AI で再取得する。
func (c *AdminCompanyController) FetchCompanyRelations(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.relationsFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	forceRefresh := ctx.QueryParam("force") == "true"
	result, err := c.relationsFetcher.FetchAndSave(ctx.Request().Context(), uint(id), forceRefresh)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_relations", "company", uint(id), map[string]any{
		"saved":  result.SavedCount,
		"force":  forceRefresh,
		"source": result.Source,
	})

	return ctx.JSON(http.StatusOK, result)
}

// ConfirmCompanyRelations POST /api/admin/companies/:id/confirm-relations
// プレビュー済みの関係・市場情報を LLM 再実行なしで DB に確定保存する。
func (c *AdminCompanyController) ConfirmCompanyRelations(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.relationsFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	var req services.CompanyRelationsResult
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	result, err := c.relationsFetcher.ConfirmAndSave(uint(id), &req)
	if err != nil {
		return echoInternalError(err)
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.confirm_relations", "company", uint(id), map[string]any{
		"saved":      result.SavedCount,
		"source":     result.Source,
		"model":      result.ModelUsed,
		"confidence": result.Confidence,
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

// FetchAllMissing POST /api/admin/companies/:id/fetch-all
// 企業管理の主取得: 基本情報 → 技術 → ビジネス関係（関係・市場）→ 求人。
// 未取得／TTL切れ／空フィールドのみ。?force=true で TTL を無視して再取得。
// 個別失敗は errors に積み、全体は 200 で返す。
func (c *AdminCompanyController) FetchAllMissing(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	forceRefresh := ctx.QueryParam("force") == "true"
	companyID := uint(id)
	reqCtx := ctx.Request().Context()

	company, err := c.repo.FindByID(companyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	payload, errs := c.runPrimaryAspectFetches(reqCtx, company, companyID, forceRefresh)
	if latest, rerr := c.repo.FindByID(companyID); rerr == nil && latest != nil {
		company = latest
	}

	// 求人（補助。失敗しても主3種の結果は返す）
	jobsStep := fetchStepResult{Status: "skipped", Skipped: true, Detail: "fetcher unavailable"}
	if c.jobFetcher != nil {
		needJobs := forceRefresh || !companyfetch.IsFresh(company.JobsFetchedAt, companyfetch.TTLJobs)
		if !needJobs {
			existing, _ := c.repo.ListJobPositions(&companyID, 100)
			if len(existing) == 0 {
				needJobs = true
			} else {
				jobsStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "ttl_fresh", Count: len(existing)}
			}
		}
		if needJobs {
			positions, jerr := c.jobFetcher.FetchAndSaveJobs(reqCtx, companyID, forceRefresh)
			if jerr != nil {
				jobsStep = fetchStepResult{Status: "error", Detail: jerr.Error()}
				errs = append(errs, "jobs: "+jerr.Error())
			} else {
				jobsStep = fetchStepResult{Status: "fetched", Detail: "ok", Count: len(positions)}
				payload["jobs_total"] = len(positions)
			}
		}
	}
	payload["jobs_step"] = jobsStep

	if len(errs) > 0 {
		payload["errors"] = errs
		payload["ok"] = false
	} else {
		payload["ok"] = true
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_all", "company", companyID, map[string]any{
		"force": forceRefresh,
		"ok":    payload["ok"],
	})

	return ctx.JSON(http.StatusOK, payload)
}

// FetchPrimary POST /api/admin/companies/:id/fetch-primary
// 主3種（基本情報・技術・ビジネス関係）を1リクエストで取得・保存する専用 API。
// 画面遷移や個別 API 呼び出しなしで、一覧/基本情報画面からまとめて取得するために使う。
// ?force=true で TTL / 既存データを無視して再取得。
func (c *AdminCompanyController) FetchPrimary(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	forceRefresh := ctx.QueryParam("force") == "true"
	companyID := uint(id)
	reqCtx := ctx.Request().Context()

	company, err := c.repo.FindByID(companyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	payload, errs := c.runPrimaryAspectFetches(reqCtx, company, companyID, forceRefresh)
	payload["aspects"] = []string{"info", "tech", "relations"}

	if len(errs) > 0 {
		payload["errors"] = errs
		payload["ok"] = false
	} else {
		payload["ok"] = true
	}

	// 最新の企業スナップショットも返す（FE が再読込なしで反映できる）
	if latest, rerr := c.repo.FindByID(companyID); rerr == nil && latest != nil {
		payload["company"] = latest
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_primary", "company", companyID, map[string]any{
		"force": forceRefresh,
		"ok":    payload["ok"],
	})

	return ctx.JSON(http.StatusOK, payload)
}

type fetchStepResult struct {
	Status  string `json:"status"` // fetched | skipped | empty | error
	Detail  string `json:"detail,omitempty"`
	Count   int    `json:"count,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// runPrimaryAspectFetches は基本・技術・ビジネス関係の取得を実行し、共通ペイロードを返す。
func (c *AdminCompanyController) runPrimaryAspectFetches(
	reqCtx context.Context,
	company *models.Company,
	companyID uint,
	forceRefresh bool,
) (map[string]any, []string) {
	payload := map[string]any{
		"company_id":   companyID,
		"company_name": company.Name,
		"force":        forceRefresh,
	}
	var errs []string

	reload := func() {
		if latest, rerr := c.repo.FindByID(companyID); rerr == nil && latest != nil {
			company = latest
		}
	}

	// 1) 基本情報
	infoStep := fetchStepResult{Status: "skipped", Skipped: true, Detail: "fetcher unavailable"}
	if c.infoFetcher != nil {
		needInfo := forceRefresh || !companyfetch.IsFresh(company.InfoFetchedAt, companyfetch.TTLInfo) ||
			!companyfetch.HasBasicInfo(company.Description, company.WebsiteURL)
		if !needInfo {
			infoStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "ttl_fresh"}
		} else {
			result, ferr := c.infoFetcher.FetchAndSave(reqCtx, companyID, forceRefresh)
			if ferr != nil {
				infoStep = fetchStepResult{Status: "error", Detail: ferr.Error()}
				errs = append(errs, "info: "+ferr.Error())
			} else if result != nil && result.FromCache {
				reload()
				// 予算超過などで中身なしのまま FromCache になると「スキップ＝成功」に見えるため、
				// 実データがあるときだけ skipped とする。
				if companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache"
					}
					infoStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: detail}
				} else {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache_without_data"
					}
					infoStep = fetchStepResult{Status: "error", Detail: detail}
					errs = append(errs, "info: cache returned without basic info ("+detail+")")
				}
				payload["info"] = result
			} else {
				reload()
				if companyfetch.HasBasicInfo(company.Description, company.WebsiteURL) {
					infoStep = fetchStepResult{Status: "fetched", Detail: "ok"}
				} else {
					infoStep = fetchStepResult{Status: "empty", Detail: "no_basic_info"}
					errs = append(errs, "info: acquired but basic info still empty")
				}
				payload["info"] = result
			}
			reload()
		}
	}
	payload["info_step"] = infoStep

	// 2) 技術
	techStep := fetchStepResult{Status: "skipped", Skipped: true, Detail: "fetcher unavailable"}
	if c.techFetcher != nil {
		needTech := forceRefresh || !companyfetch.IsFresh(company.TechFetchedAt, companyfetch.TTLTech) ||
			!companyfetch.HasTechData(company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle)
		if !needTech {
			techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "ttl_fresh"}
		} else {
			result, terr := c.techFetcher.FetchAndSave(reqCtx, companyID, forceRefresh)
			if terr != nil {
				techStep = fetchStepResult{Status: "error", Detail: terr.Error()}
				errs = append(errs, "tech: "+terr.Error())
			} else if result != nil && result.FromCache {
				reload()
				if companyfetch.HasTechData(company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache"
					}
					techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: detail}
				} else {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache_without_data"
					}
					techStep = fetchStepResult{Status: "error", Detail: detail}
					errs = append(errs, "tech: cache returned without tech_stack ("+detail+")")
				}
				payload["tech"] = result
			} else {
				reload()
				if companyfetch.HasTechData(company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
					techStep = fetchStepResult{Status: "fetched", Detail: "ok"}
				} else {
					techStep = fetchStepResult{Status: "empty", Detail: "no_tech_stack"}
					errs = append(errs, "tech: acquired but tech_stack still empty")
				}
				payload["tech"] = result
			}
			reload()
		}
	}
	payload["tech_step"] = techStep

	// 3) ビジネス関係
	relationsStep := fetchStepResult{Status: "skipped", Skipped: true, Detail: "fetcher unavailable"}
	if c.relationsFetcher != nil {
		needRel := forceRefresh || !companyfetch.IsFresh(company.RelationsFetchedAt, companyfetch.TTLRelations) ||
			!c.relationsFetcher.HasStoredData(companyID)
		if !needRel {
			relationsStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "ttl_fresh"}
		} else {
			result, rerr := c.relationsFetcher.FetchAndSave(reqCtx, companyID, forceRefresh)
			if rerr != nil {
				relationsStep = fetchStepResult{Status: "error", Detail: rerr.Error()}
				errs = append(errs, "relations: "+rerr.Error())
			} else if result != nil && result.FromCache {
				if c.relationsFetcher.HasStoredData(companyID) {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache"
					}
					relationsStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: detail, Count: result.SavedCount}
				} else {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache_without_data"
					}
					relationsStep = fetchStepResult{Status: "error", Detail: detail, Count: result.SavedCount}
					errs = append(errs, "relations: cache returned without stored data ("+detail+")")
				}
				payload["relations"] = result
			} else {
				count := 0
				if result != nil {
					count = result.SavedCount
				}
				if count > 0 || c.relationsFetcher.HasStoredData(companyID) {
					relationsStep = fetchStepResult{Status: "fetched", Detail: "ok", Count: count}
				} else {
					relationsStep = fetchStepResult{Status: "empty", Detail: "no_relations", Count: 0}
					errs = append(errs, "relations: acquired but no relations/market saved")
				}
				payload["relations"] = result
			}
			reload()
		}
	}
	payload["relations_step"] = relationsStep

	return payload, errs
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

// FetchMissingBatch POST /api/admin/companies/fetch-missing-batch
// Body: { "limit": 20, "dry_run": true }
// アクティブ企業のうち不足フィールド（基本情報/求人/Tech/関係）だけを上限付きで埋める。
func (c *AdminCompanyController) FetchMissingBatch(ctx echo.Context) error {
	if c.missingBatch == nil {
		c.missingBatch = services.NewCompanyMissingBatchService(
			c.repo, c.infoFetcher, c.jobFetcher, c.techFetcher, c.relationsFetcher,
		)
	}
	var opts services.MissingBatchOptions
	opts.Limit = 20
	if err := ctx.Bind(&opts); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	result, err := c.missingBatch.Run(ctx.Request().Context(), opts)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.fetch_missing_batch", "company", 0, map[string]any{
		"dry_run":       result.DryRun,
		"limit":         result.Limit,
		"candidate_n":   result.CandidateN,
		"processed":     result.Processed,
		"info_ok":       result.InfoOK,
		"jobs_ok":       result.JobsOK,
		"tech_ok":       result.TechOK,
		"relations_ok":  result.RelationsOK,
		"errors":        result.Errors,
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
