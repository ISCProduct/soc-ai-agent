// admin_company_fetch_controller.go — AI を使った企業情報・関係・求人・ペルソナの
// 取得・確定保存エンドポイント群（AdminCompanyController のメソッド）。

package controllers

import (
	"Backend/internal/companyfetch"
	"Backend/internal/companyfields"
	"Backend/internal/models"
	"Backend/internal/services/company"
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

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

	var req company.CompanyInfoResult
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
// ?cache_only=true を付けると DB の保存済みデータのみ返す（AI 再取得なし）。
func (c *AdminCompanyController) FetchCompanyRelations(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.relationsFetcher == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}

	if ctx.QueryParam("cache_only") == "true" {
		result, err := c.relationsFetcher.LoadSaved(uint(id))
		if err != nil {
			return echoInternalError(err)
		}
		return ctx.JSON(http.StatusOK, result)
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

	var req company.CompanyRelationsResult
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
			existing, _ := c.repo.ListJobPositions(&companyID, nil, 100)
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
		// FE missingAspects（概要+公式URL）と揃え、スタンプ済みでも中身不足なら再取得する。
		needInfo := forceRefresh ||
			!companyfetch.IsFresh(company.InfoFetchedAt, companyfetch.TTLInfo) ||
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
				} else if companyfetch.HasBasicInfoFootprint(company.Description, company.WebsiteURL, company.Location) {
					// AI/gBiz は成功したが概要など公開事実が無い → 未取得ではなく確認済み（公開情報限定）
					infoStep = fetchStepResult{Status: "empty", Detail: "public_info_sparse"}
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

	// 2) 技術（業界プロファイルで不要な場合はスキップ）
	techStep := fetchStepResult{Status: "skipped", Skipped: true, Detail: "fetcher unavailable"}
	if !companyfields.TechAspectEnabled(company.Industry) {
		techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "industry_not_applicable"}
	} else if c.techFetcher != nil {
		needTech := forceRefresh || !companyfetch.IsFresh(company.TechFetchedAt, companyfetch.TTLTech) ||
			!companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle)
		if !needTech {
			techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "ttl_fresh"}
		} else {
			result, terr := c.techFetcher.FetchAndSave(reqCtx, companyID, forceRefresh)
			if terr != nil {
				techStep = fetchStepResult{Status: "error", Detail: terr.Error()}
				errs = append(errs, "tech: "+terr.Error())
			} else if result != nil && result.FromCache {
				reload()
				if companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
					detail := result.SkipReason
					if detail == "" {
						detail = "cache"
					}
					techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: detail}
				} else if !companyfields.RequiresTech(company.Industry) {
					// 公開必須でない業種は空でも失敗扱いにしない
					techStep = fetchStepResult{Status: "skipped", Skipped: true, Detail: "optional_empty"}
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
				if companyfetch.HasTechDataForIndustry(company.Industry, company.TechStack, company.InfraStack, company.CicdTools, company.DevelopmentStyle) {
					techStep = fetchStepResult{Status: "fetched", Detail: "ok"}
				} else {
					status, detail, asErr := companyfields.TechEmptyStepStatus(company.Industry)
					techStep = fetchStepResult{Status: status, Skipped: status == "skipped", Detail: detail}
					if asErr {
						errs = append(errs, "tech: acquired but tech_stack still empty")
					}
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
					detail := "ok"
					if result != nil && len(result.Relations) == 0 && result.MarketInfo != nil &&
						companyfetch.IsConfirmedUnlisted(result.MarketInfo.IsListed, result.MarketInfo.MarketType, result.MarketInfo.StockCode) {
						detail = "confirmed_unlisted"
					}
					relationsStep = fetchStepResult{Status: "fetched", Detail: detail, Count: count}
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
