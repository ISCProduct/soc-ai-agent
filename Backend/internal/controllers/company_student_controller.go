package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/middleware"
	"Backend/internal/repositories"
	hrsvc "Backend/internal/services/hr"

	"github.com/labstack/echo/v4"
)

// studentSearchService / studentVisibleAnalysisService はテストで差し替えるための
// 狭いインターフェース（hr_student_analysis_controller.go と同じ方針）。
type studentSearchService interface {
	Search(companyID uint, f repositories.StudentSearchFilters) (*hrsvc.StudentSearchResult, error)
	SemanticSearch(ctx context.Context, companyID uint, query string, f repositories.StudentSearchFilters) (*hrsvc.StudentSearchResult, error)
	AddTag(companyID, companyUserID, userID uint, tagName string) error
	RemoveTag(companyID, tagID uint) error
	ListTagNames(companyID uint) ([]string, error)
	ListTagsForUser(companyID, userID uint) ([]hrsvc.StudentTagView, error)
}

type industryLister interface {
	ListActive() ([]repositories.IndustryOption, error)
}

type studentVisibleAnalysisService interface {
	GetAnalysisForVisibleStudent(targetUserID uint) (*hrsvc.StudentAnalysisResponse, error)
}

// CompanyStudentController は企業ポータルの学生検索・タグ管理（#1094）。
// 全エンドポイントが企業ポータルJWTの company_id でスコープされる。
type CompanyStudentController struct {
	search     studentSearchService
	analysis   studentVisibleAnalysisService
	industries industryLister
}

func NewCompanyStudentController(
	search studentSearchService,
	analysis studentVisibleAnalysisService,
	industries industryLister,
) *CompanyStudentController {
	return &CompanyStudentController{search: search, analysis: analysis, industries: industries}
}

// Industries GET /api/company-portal/industries
// 業界フィルタの選択肢。企業固有のデータは含まないため company_id によるスコープは不要。
func (c *CompanyStudentController) Industries(ctx echo.Context) error {
	if _, ok := echoCompanyID(ctx); !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	options, err := c.industries.ListActive()
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": options})
}

// echoCompanyID は認証済み企業ユーザーの所属 company_id を返す。
func echoCompanyID(c echo.Context) (uint, bool) {
	return middleware.CompanyIDFromContext(c.Request().Context())
}

func (c *CompanyStudentController) filtersFrom(ctx echo.Context) repositories.StudentSearchFilters {
	industryID, _ := strconv.ParseUint(ctx.QueryParam("industry_id"), 10, 64)
	return repositories.StudentSearchFilters{
		IndustryID: uint(industryID),
		Location:   ctx.QueryParam("location"),
		Skill:      ctx.QueryParam("skill"),
		Tag:        ctx.QueryParam("tag"),
		Limit:      echoIntQuery(ctx, "limit", 30),
		Offset:     echoIntQuery(ctx, "offset", 0),
	}
}

// List GET /api/company-portal/students
func (c *CompanyStudentController) List(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	result, err := c.search.Search(companyID, c.filtersFrom(ctx))
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, result)
}

type semanticSearchBody struct {
	Query string `json:"query"`
}

// SemanticSearch POST /api/company-portal/students/semantic-search
func (c *CompanyStudentController) SemanticSearch(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	var body semanticSearchBody
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	result, err := c.search.SemanticSearch(ctx.Request().Context(), companyID, body.Query, c.filtersFrom(ctx))
	if err != nil {
		return mapStudentSearchError(err)
	}
	return ctx.JSON(http.StatusOK, result)
}

// Detail GET /api/company-portal/students/:userID
// 学生の分析プロファイル（#1096）に自社タグを添えて返す。
func (c *CompanyStudentController) Detail(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	userID, err := echoUintParam(ctx, "userID")
	if err != nil {
		return err
	}
	analysis, err := c.analysis.GetAnalysisForVisibleStudent(userID)
	if err != nil {
		return mapStudentSearchError(err)
	}
	tags, err := c.search.ListTagsForUser(companyID, userID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"analysis": analysis,
		"tags":     tags,
	})
}

type addTagBody struct {
	TagName string `json:"tag_name"`
}

// AddTag POST /api/company-portal/students/:userID/tags
func (c *CompanyStudentController) AddTag(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	companyUserID, ok := echoCompanyUserID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	userID, err := echoUintParam(ctx, "userID")
	if err != nil {
		return err
	}
	var body addTagBody
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	if err := c.search.AddTag(companyID, companyUserID, userID, body.TagName); err != nil {
		return mapStudentSearchError(err)
	}
	tags, err := c.search.ListTagsForUser(companyID, userID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"tags": tags})
}

// RemoveTag DELETE /api/company-portal/students/:userID/tags/:tagID
func (c *CompanyStudentController) RemoveTag(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	tagID, err := echoUintParam(ctx, "tagID")
	if err != nil {
		return err
	}
	if err := c.search.RemoveTag(companyID, tagID); err != nil {
		return echoInternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// ListTags GET /api/company-portal/tags
func (c *CompanyStudentController) ListTags(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	names, err := c.search.ListTagNames(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": names})
}

func mapStudentSearchError(err error) error {
	switch {
	case errors.Is(err, hrsvc.ErrStudentNotVisible):
		return newAPIError(http.StatusNotFound, ErrCodeNotFound, "学生が見つかりません")
	case errors.Is(err, hrsvc.ErrInvalidTagName), errors.Is(err, hrsvc.ErrEmptyQuery):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "入力内容が不正です")
	case errors.Is(err, hrsvc.ErrSemanticSearchUnavailable):
		return newAPIError(http.StatusServiceUnavailable, ErrCodeServiceUnavail, "セマンティック検索を利用できません")
	default:
		return echoInternalError(err)
	}
}
