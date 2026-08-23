package controllers

import (
	"Backend/internal/services/interfaces"
	"Backend/internal/services/shared"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// ApplicationController 応募・選考ステータス管理コントローラー
type ApplicationController struct {
	appService interfaces.ApplicationService
}

func NewApplicationController(appService interfaces.ApplicationService) *ApplicationController {
	return &ApplicationController{appService: appService}
}

// applicationErrorStatus はサービス層エラーメッセージの先頭コード（application_service.go参照。
// docs/requirements/application-status-transition.md §11 準拠）と対応するHTTPステータス。
var applicationErrorStatus = map[string]int{
	"application_not_found":        http.StatusNotFound,
	"forbidden":                    http.StatusForbidden,
	"application_already_closed":   http.StatusConflict,
	"invalid_status_transition":    http.StatusConflict,
	"application_not_offered":      http.StatusConflict,
	"duplicate_active_application": http.StatusConflict,
}

// mapApplicationError はサービス層エラー（"コード: 詳細" 形式）を、JSONの code フィールドが
// 一致する echo.HTTPError に変換する。未知のエラーは 400 Bad Request にフォールバックする。
func mapApplicationError(err error) error {
	msg := err.Error()
	for code, status := range applicationErrorStatus {
		if strings.HasPrefix(msg, code+":") {
			return newAPIError(status, code, msg)
		}
	}
	return echo.NewHTTPError(http.StatusBadRequest, msg)
}

// Apply POST /api/applications - 企業への応募登録
func (c *ApplicationController) Apply(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}

	var req struct {
		UserID    uint `json:"user_id"` // 互換のため受け取るが無視する
		CompanyID uint `json:"company_id"`
		MatchID   uint `json:"match_id"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.CompanyID == 0 || req.MatchID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id, match_id は必須です")
	}

	app, err := c.appService.Apply(userID, req.CompanyID, req.MatchID)
	if err != nil {
		switch {
		case errors.Is(err, shared.ErrNotFound):
			return echo.NewHTTPError(http.StatusNotFound, "match not found")
		case errors.Is(err, shared.ErrForbidden):
			return echo.NewHTTPError(http.StatusForbidden, "他人のmatchには応募できません")
		case errors.Is(err, shared.ErrDuplicateActiveApplication):
			return echo.NewHTTPError(http.StatusConflict, "duplicate_active_application")
		default:
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
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
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}

	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req struct {
		UserID  uint   `json:"user_id"` // 互換のため受け取るが無視する
		Status  string `json:"status"`
		Notes   string `json:"notes"`
		IsAdmin bool   `json:"is_admin"` // クライアント指定は無視（権限昇格防止）
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.Status == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "status は必須です")
	}

	app, err := c.appService.UpdateStatus(uint(id), userID, req.Status, req.Notes, false)
	if err != nil {
		return mapApplicationError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     app.ID,
		"status": app.Status,
		"notes":  app.Notes,
	})
}

// AdminUpdateStatus PATCH /api/admin/applications/{id}/status - 管理者による選考ステータス更新。
// isAdmin は常に true 固定（クライアント入力からは取らない。#1016）。
func (c *ApplicationController) AdminUpdateStatus(ctx echo.Context) error {
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.Status == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "status は必須です")
	}

	app, err := c.appService.UpdateStatus(id, 0, req.Status, req.Notes, true)
	if err != nil {
		return mapApplicationError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     app.ID,
		"status": app.Status,
		"notes":  app.Notes,
	})
}

// Withdraw POST /api/applications/{id}/withdraw - ユーザーによる選考辞退（§10.3）
func (c *ApplicationController) Withdraw(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}

	app, err := c.appService.Withdraw(id, userID, false)
	if err != nil {
		return mapApplicationError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     app.ID,
		"status": app.Status,
	})
}

// Accept POST /api/applications/{id}/accept - ユーザーによる内定承諾（§10.4）
func (c *ApplicationController) Accept(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}
	id, err := echoUintParam(ctx, "id")
	if err != nil {
		return err
	}

	app, err := c.appService.Accept(id, userID, false)
	if err != nil {
		return mapApplicationError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     app.ID,
		"status": app.Status,
	})
}

// AdminList GET /api/admin/applications - 管理者による選考一覧取得。
// user_id / company_id / status クエリパラメータで絞り込める（§10.5）。
func (c *ApplicationController) AdminList(ctx echo.Context) error {
	var userID, companyID uint
	if v := ctx.QueryParam("user_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
		}
		userID = uint(id)
	}
	if v := ctx.QueryParam("company_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid company_id")
		}
		companyID = uint(id)
	}
	status := ctx.QueryParam("status")

	apps, err := c.appService.ListForAdmin(userID, companyID, status)
	if err != nil {
		return echoInternalError(err)
	}

	type AdminAppResponse struct {
		ID          uint   `json:"id"`
		UserID      uint   `json:"user_id"`
		CompanyID   uint   `json:"company_id"`
		CompanyName string `json:"company_name"`
		Status      string `json:"status"`
		Notes       string `json:"notes"`
		AppliedAt   any    `json:"applied_at"`
	}

	resp := make([]AdminAppResponse, len(apps))
	for i, app := range apps {
		name := ""
		if app.Company != nil {
			name = app.Company.Name
		}
		resp[i] = AdminAppResponse{
			ID:          app.ID,
			UserID:      app.UserID,
			CompanyID:   app.CompanyID,
			CompanyName: name,
			Status:      app.Status,
			Notes:       app.Notes,
			AppliedAt:   app.AppliedAt,
		}
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"applications": resp,
		"total":        len(resp),
	})
}

// List GET /api/applications - 認証ユーザーの応募一覧取得
func (c *ApplicationController) List(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}

	apps, err := c.appService.GetApplicationsByUser(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "データ取得エラー")
	}

	type AppResponse struct {
		ID              uint `json:"id"`
		CompanyID       uint `json:"company_id"`
		CompanyName     string `json:"company_name"`
		CompanyIndustry string `json:"company_industry"`
		MatchID         uint `json:"match_id"`
		Status          string `json:"status"`
		Notes           string `json:"notes"`
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
	if _, ok := echoUserID(ctx); !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}

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
