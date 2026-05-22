package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type AdminCompanyController struct {
	repo         repository.CompanyRepository
	audit        *services.AuditLogService
	gbiz         *services.GBizInfoService
	openaiClient *openai.Client
}

func NewAdminCompanyController(repo repository.CompanyRepository, audit *services.AuditLogService, gbiz *services.GBizInfoService, openaiClient ...*openai.Client) *AdminCompanyController {
	ctrl := &AdminCompanyController{repo: repo, audit: audit, gbiz: gbiz}
	if len(openaiClient) > 0 {
		ctrl.openaiClient = openaiClient[0]
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

// FetchTechStack POST /api/admin/companies/:id/tech-stack-search
// OpenAI WebSearchで企業の技術スタックを取得してDBを更新する
func (c *AdminCompanyController) FetchTechStack(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	if c.openaiClient == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "openai client not configured")
	}
	company, err := c.repo.FindByID(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "company not found")
	}

	prompt := fmt.Sprintf(
		`「%s」という日本のIT企業の技術スタックを調査してください。以下のJSON形式のみで回答してください（余分な説明は不要）。
{
  "tech_stack": ["言語・フレームワーク名（例: Go, React, TypeScript）"],
  "infra_stack": ["インフラ名（例: AWS, GCP, Azure, オンプレ）"],
  "cicd_tools": ["CI/CDツール名（例: GitHub Actions, Jenkins, CircleCI）"],
  "development_style": "開発手法（例: スクラム, ウォーターフォール, カンバン）"
}`,
		company.Name,
	)

	reqCtx, cancel := context.WithTimeout(ctx.Request().Context(), 30*time.Second)
	defer cancel()

	text, err := c.openaiClient.WebSearchQuery(reqCtx, prompt)
	if err != nil {
		return echoInternalError(err)
	}

	// JSON部分を抽出
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to parse web search response")
	}

	type techStackResult struct {
		TechStack        []string `json:"tech_stack"`
		InfraStack       []string `json:"infra_stack"`
		CicdTools        []string `json:"cicd_tools"`
		DevelopmentStyle string   `json:"development_style"`
	}
	var result techStackResult
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to parse tech stack json")
	}

	// JSON配列をシリアライズしてDBに保存
	if len(result.TechStack) > 0 {
		if b, err := json.Marshal(result.TechStack); err == nil {
			company.TechStack = string(b)
		}
	}
	if len(result.InfraStack) > 0 {
		if b, err := json.Marshal(result.InfraStack); err == nil {
			company.InfraStack = string(b)
		}
	}
	if len(result.CicdTools) > 0 {
		if b, err := json.Marshal(result.CicdTools); err == nil {
			company.CicdTools = string(b)
		}
	}
	if result.DevelopmentStyle != "" {
		company.DevelopmentStyle = result.DevelopmentStyle
	}

	if err := c.repo.Update(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update company")
	}

	actor := ctx.Request().Header.Get("X-Admin-Email")
	c.audit.Record(actor, "company.tech_stack_search", "company", company.ID, map[string]any{
		"name": company.Name,
	})

	return ctx.JSON(http.StatusOK, map[string]any{
		"tech_stack":        result.TechStack,
		"infra_stack":       result.InfraStack,
		"cicd_tools":        result.CicdTools,
		"development_style": result.DevelopmentStyle,
	})
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
