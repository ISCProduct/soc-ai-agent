package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// ApplicationController 応募・選考ステータス管理コントローラー
type ApplicationController struct {
	appService *services.ApplicationService
}

func NewApplicationController(appService *services.ApplicationService) *ApplicationController {
	return &ApplicationController{appService: appService}
}

// Apply POST /api/applications - 企業への応募登録
func (c *ApplicationController) Apply(ctx echo.Context) error {
	var req struct {
		UserID    uint `json:"user_id"`
		CompanyID uint `json:"company_id"`
		MatchID   uint `json:"match_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.UserID == 0 || req.CompanyID == 0 || req.MatchID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id, company_id, match_id は必須です")
	}

	app, err := c.appService.Apply(req.UserID, req.CompanyID, req.MatchID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusCreated, map[string]any{
		"id":         app.ID,
		"user_id":    app.UserID,
		"company_id": app.CompanyID,
		"match_id":   app.MatchID,
		"status":     app.Status,
		"applied_at": app.AppliedAt,
	})
}

// UpdateStatus PUT /api/applications/{id} - 選考ステータス更新
func (c *ApplicationController) UpdateStatus(ctx echo.Context) error {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req struct {
		UserID uint   `json:"user_id"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.UserID == 0 || req.Status == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id と status は必須です")
	}

	app, err := c.appService.UpdateStatus(uint(id), req.UserID, req.Status, req.Notes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     app.ID,
		"status": app.Status,
		"notes":  app.Notes,
	})
}

// List GET /api/applications?user_id=X - ユーザーの応募一覧取得
func (c *ApplicationController) List(ctx echo.Context) error {
	userIDStr := ctx.QueryParam("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil || userID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id は必須です")
	}

	apps, err := c.appService.GetApplicationsByUser(uint(userID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "データ取得エラー")
	}

	type AppResponse struct {
		ID              uint        `json:"id"`
		CompanyID       uint        `json:"company_id"`
		CompanyName     string      `json:"company_name"`
		CompanyIndustry string      `json:"company_industry"`
		MatchID         uint        `json:"match_id"`
		Status          string      `json:"status"`
		Notes           string      `json:"notes"`
		AppliedAt       any `json:"applied_at"`
		StatusUpdatedAt any `json:"status_updated_at"`
	}

	resp := make([]AppResponse, len(apps))
	for i, app := range apps {
		name := ""
		industry := ""
		if app.Company != nil {
			name = app.Company.Name
			industry = app.Company.Industry
		}
		resp[i] = AppResponse{
			ID:              app.ID,
			CompanyID:       app.CompanyID,
			CompanyName:     name,
			CompanyIndustry: industry,
			MatchID:         app.MatchID,
			Status:          app.Status,
			Notes:           app.Notes,
			AppliedAt:       app.AppliedAt,
			StatusUpdatedAt: app.StatusUpdatedAt,
		}
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"applications": resp,
		"total":        len(resp),
	})
}

// GetCorrelation GET /api/applications/correlation?company_id=X - 相関分析データ取得
func (c *ApplicationController) GetCorrelation(ctx echo.Context) error {
	companyIDStr := ctx.QueryParam("company_id")
	var companyID uint
	if companyIDStr != "" {
		id, err := strconv.ParseUint(companyIDStr, 10, 64)
		if err == nil {
			companyID = uint(id)
		}
	}

	data, err := c.appService.GetCorrelation(companyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "相関データ取得エラー")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"correlation": data,
		"total":       len(data),
	})
}
