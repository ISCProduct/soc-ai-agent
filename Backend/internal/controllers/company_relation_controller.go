package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/internal/services/shared"
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

type CompanyRelationController struct {
	repo         repository.CompanyRelationQueryRepository
	openaiClient *openai.Client
	validator    *company.CompanyValidationService
}

func NewCompanyRelationController(repo repository.CompanyRelationQueryRepository, openaiClient *openai.Client) *CompanyRelationController {
	return &CompanyRelationController{repo: repo, openaiClient: openaiClient}
}

func (ctrl *CompanyRelationController) SetCompanyValidator(v *company.CompanyValidationService) {
	ctrl.validator = v
}

// GetCompanyRelations 企業IDに関連する企業関係を取得
// GET /api/companies/:id/relations
func (ctrl *CompanyRelationController) GetCompanyRelations(ctx echo.Context) error {
	companyID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	relations, err := ctrl.repo.GetByCompanyID(uint(companyID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch relations")
	}

	return ctx.JSON(http.StatusOK, relations)
}

// GetCompanyMarketInfo 企業の市場情報を取得
// GET /api/companies/:id/market-info
func (ctrl *CompanyRelationController) GetCompanyMarketInfo(ctx echo.Context) error {
	companyID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	marketInfo, err := ctrl.repo.GetMarketInfoByCompanyID(uint(companyID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch market info")
	}
	if marketInfo == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Market info not found")
	}

	return ctx.JSON(http.StatusOK, marketInfo)
}

// GetAllCompanyRelations 全企業関係を取得（関連図用）
// GET /api/companies/relations
func (ctrl *CompanyRelationController) GetAllCompanyRelations(ctx echo.Context) error {
	relations, err := ctrl.repo.GetAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch relations")
	}

	return ctx.JSON(http.StatusOK, relations)
}

// GetAllMarketInfo 全企業の市場情報を取得
// GET /api/companies/market-info
func (ctrl *CompanyRelationController) GetAllMarketInfo(ctx echo.Context) error {
	marketInfos, err := ctrl.repo.GetAllMarketInfo()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch market info")
	}

	return ctx.JSON(http.StatusOK, marketInfos)
}

// GetCompanyByID 企業IDで企業詳細を取得
// GET /api/companies/:id
func (ctrl *CompanyRelationController) GetCompanyByID(ctx echo.Context) error {
	companyID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	company, err := ctrl.repo.GetCompanyByID(uint(companyID))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Company not found")
	}

	return ctx.JSON(http.StatusOK, company)
}

// GetCompanyJobPositions 企業の公開済み求人一覧を取得
// GET /api/companies/:id/job-positions
func (ctrl *CompanyRelationController) GetCompanyJobPositions(ctx echo.Context) error {
	companyID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	positions, err := ctrl.repo.GetJobPositionsByCompany(uint(companyID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch job positions")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"positions": positions,
	})
}

// GetCompanies 企業一覧を取得
// GET /api/companies
func (ctrl *CompanyRelationController) GetCompanies(ctx echo.Context) error {
	limitStr := ctx.QueryParam("limit")
	offsetStr := ctx.QueryParam("offset")
	industry := ctx.QueryParam("industry")
	name := ctx.QueryParam("name")
	tech := ctx.QueryParam("tech")

	limit := 10 // デフォルト
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 100 {
				limit = 100 // 最大100件
			}
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	companies, total, err := ctrl.repo.GetCompaniesFiltered(limit, offset, industry, name, tech)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch companies")
	}

	type CompanyResponse struct {
		Companies any   `json:"companies"`
		Total     int64 `json:"total"`
		Limit     int   `json:"limit"`
		Offset    int   `json:"offset"`
	}

	response := CompanyResponse{
		Companies: companies,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	}

	return ctx.JSON(http.StatusOK, response)
}

// GetCompanyBrief 共有キャッシュから壁打ち/面接用スナップショットを返す（Search なし）
// GET /api/companies/brief?name=xxx
func (ctrl *CompanyRelationController) GetCompanyBrief(ctx echo.Context) error {
	name := strings.TrimSpace(ctx.QueryParam("name"))
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	companies, _, err := ctrl.repo.GetCompaniesFiltered(1, 0, "", name, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch company")
	}
	if len(companies) == 0 {
		return ctx.JSON(http.StatusOK, map[string]any{
			"brief": "",
			"found": false,
		})
	}
	c := companies[0]
	return ctx.JSON(http.StatusOK, map[string]any{
		"brief":      company.BuildCompanyBrief(&c, nil),
		"found":      true,
		"company_id": c.ID,
		"name":       c.Name,
	})
}

// WebSearchCompanies OpenAI Web Searchを使用して企業をWEB検索
// GET /api/companies/web-search?q=xxx
func (ctrl *CompanyRelationController) WebSearchCompanies(ctx echo.Context) error {
	query := strings.TrimSpace(ctx.QueryParam("q"))
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q parameter is required")
	}

	if ctrl.validator != nil {
		results, err := ctrl.validator.SearchCandidates(ctx.Request().Context(), query, true)
		if err != nil {
			var ve *shared.ValidationError
			if errors.As(err, &ve) {
				return echo.NewHTTPError(http.StatusBadRequest, ve.Message)
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "企業検索に失敗しました")
		}
		out := make([]map[string]any, 0, len(results))
		for _, r := range results {
			item := map[string]any{
				"name":        r.CanonicalName,
				"description": r.Description,
				"source":      r.Source,
				"exists":      r.Exists,
				"confidence":  r.Confidence,
			}
			if r.CompanyID != nil {
				item["company_id"] = *r.CompanyID
			}
			if len(r.EvidenceURLs) > 0 {
				item["evidence_urls"] = r.EvidenceURLs
			}
			out = append(out, item)
		}
		return ctx.JSON(http.StatusOK, map[string]any{"results": out})
	}

	results := ctrl.searchCompaniesWithOpenAI(ctx.Request().Context(), query)
	return ctx.JSON(http.StatusOK, map[string]any{"results": results})
}

// ValidateCompany POST /api/companies/validate
// 企業名の実在確認（DB優先、必要時のみ軽量 WebSearch）
func (ctrl *CompanyRelationController) ValidateCompany(ctx echo.Context) error {
	if ctrl.validator == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "company validator is not configured")
	}
	var payload struct {
		Query string `json:"query"`
	}
	if err := ctx.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	result, err := ctrl.validator.Validate(ctx.Request().Context(), payload.Query)
	if err != nil {
		var ve *shared.ValidationError
		if errors.As(err, &ve) {
			return echo.NewHTTPError(http.StatusBadRequest, ve.Message)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, result)
}

// searchCompaniesWithOpenAI はAIモデルの知識から企業候補を取得する（validator未設定時のフォールバック）
func (ctrl *CompanyRelationController) searchCompaniesWithOpenAI(ctx context.Context, query string) []map[string]string {
	systemPrompt := `あなたは日本企業に詳しいアシスタントです。確実に知っている実在の企業のみを回答し、不明な場合は空配列にしてください。推測で企業を作らないでください。`
	userPrompt := fmt.Sprintf(
		`「%s」という検索キーワードに合致する日本の企業を最大5件挙げてください。キーワードと一致する企業が実在する場合は必ず最初に含めてください。以下のJSON形式のみで返してください。余分な説明は不要です。
{"results":[{"name":"企業名","description":"事業内容の1行説明"}]}`,
		query,
	)

	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	text, err := ctrl.openaiClient.ChatCompletionJSON(ctxTimeout, systemPrompt, userPrompt, 0.2, 500)
	if err != nil {
		return []map[string]string{}
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return []map[string]string{}
	}

	var parsed struct {
		Results []map[string]string `json:"results"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &parsed); err != nil {
		return []map[string]string{}
	}
	if parsed.Results == nil {
		return []map[string]string{}
	}
	return parsed.Results
}
