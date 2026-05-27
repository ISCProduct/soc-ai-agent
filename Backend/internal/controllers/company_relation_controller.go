package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/openai"
	"context"
	"encoding/json"
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
}

func NewCompanyRelationController(repo repository.CompanyRelationQueryRepository, openaiClient *openai.Client) *CompanyRelationController {
	return &CompanyRelationController{repo: repo, openaiClient: openaiClient}
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
		Companies any `json:"companies"`
		Total     int64       `json:"total"`
		Limit     int         `json:"limit"`
		Offset    int         `json:"offset"`
	}

	response := CompanyResponse{
		Companies: companies,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	}

	return ctx.JSON(http.StatusOK, response)
}

// WebSearchCompanies OpenAI Web Searchを使用して企業をWEB検索
// GET /api/companies/web-search?q=xxx
func (ctrl *CompanyRelationController) WebSearchCompanies(ctx echo.Context) error {
	query := strings.TrimSpace(ctx.QueryParam("q"))
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q parameter is required")
	}

	results := ctrl.searchCompaniesWithOpenAI(ctx.Request().Context(), query)

	return ctx.JSON(http.StatusOK, map[string]any{"results": results})
}

// searchCompaniesWithOpenAI はOpenAI Web Search APIを使って企業候補を取得する
func (ctrl *CompanyRelationController) searchCompaniesWithOpenAI(ctx context.Context, query string) []map[string]string {
	prompt := fmt.Sprintf(
		`「%s」という検索キーワードで日本の企業を最大5件検索してください。キーワードと一致する企業が実在する場合は必ず最初に含めてください。以下のJSON形式のみで返してください。余分な説明は不要です。
[{"name":"企業名","description":"事業内容の1行説明"}]`,
		query,
	)

	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	text, err := ctrl.openaiClient.WebSearchQuery(ctxTimeout, prompt)
	if err != nil {
		return []map[string]string{}
	}

	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start == -1 || end == -1 || end <= start {
		return []map[string]string{}
	}

	var results []map[string]string
	if err := json.Unmarshal([]byte(text[start:end+1]), &results); err != nil {
		return []map[string]string{}
	}
	return results
}

// splitPath はURLパスを "/" で分割してスラッシュを除去した要素のスライスを返す（後方互換性のため残存）
func splitPath(path string) []string {
	result := []string{}
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				result = append(result, path[start:i])
			}
			start = i + 1
		}
	}
	return result
}

// trimSpace は文字列の先頭と末尾の空白を除去する
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
