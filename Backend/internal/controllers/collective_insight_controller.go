package controllers

import (
	"Backend/internal/services"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// CollectiveInsightController 集合知レコメンドAPI
type CollectiveInsightController struct {
	svc *services.CollectiveInsightService
}

func NewCollectiveInsightController(svc *services.CollectiveInsightService) *CollectiveInsightController {
	return &CollectiveInsightController{svc: svc}
}

// GetRecommendations GET /api/collective-insights/recommendations?session_id=xxx
// 類似スコアプロファイルのユーザーが通過した企業をレコメンドする
func (c *CollectiveInsightController) GetRecommendations(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	// 除外企業IDをオプションで受け取る（カンマ区切り）
	var excludeIDs []uint
	if excStr := ctx.QueryParam("exclude"); excStr != "" {
		for idStr := range strings.SplitSeq(excStr, ",") {
			if id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32); err == nil {
				excludeIDs = append(excludeIDs, uint(id))
			}
		}
	}

	items, err := c.svc.GetCollectiveRecommendations(userID, sessionID, excludeIDs)
	if err != nil {
		return echoInternalError(err)
	}
	if items == nil {
		items = []services.CollectiveRecommendItem{}
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"recommendations": items,
		"count":           len(items),
	})
}

// GetTopPassRateCompanies GET /api/collective-insights/top-companies?limit=10
// 全ユーザー通過率の高い企業ランキング
func (c *CollectiveInsightController) GetTopPassRateCompanies(ctx echo.Context) error {
	limit := 10
	if l := ctx.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	companies, err := c.svc.GetTopPassRateCompanies(limit)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"companies": companies,
	})
}

// UpdateConsent PUT /api/collective-insights/consent
// ユーザーの集合知参加同意を更新する
func (c *CollectiveInsightController) UpdateConsent(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		Allow bool `json:"allow"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := c.svc.UpdateConsent(userID, req.Allow); err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"user_id": userID,
		"allow":   req.Allow,
	})
}

// RecordAction POST /api/collective-insights/actions
// ユーザー行動を匿名ログとして記録する
func (c *CollectiveInsightController) RecordAction(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	var req struct {
		SessionID  string `json:"session_id"`
		CompanyID  uint   `json:"company_id"`
		ActionType string `json:"action_type"` // viewed / applied / passed / rejected
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.CompanyID == 0 || req.ActionType == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "company_id, action_type are required")
	}

	validActions := map[string]bool{"viewed": true, "applied": true, "passed": true, "rejected": true}
	if !validActions[req.ActionType] {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid action_type")
	}

	if err := c.svc.RecordAction(userID, req.SessionID, req.CompanyID, req.ActionType); err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusCreated, map[string]any{"status": "recorded"})
}

// RebuildSummaries POST /api/admin/collective-insights/rebuild-summaries
// 全企業の行動サマリーをバッチ再集計する（管理画面用）
func (c *CollectiveInsightController) RebuildSummaries(ctx echo.Context) error {
	if err := c.svc.RebuildSummaries(); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"status": "rebuilt"})
}
