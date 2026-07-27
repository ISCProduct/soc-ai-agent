package controllers

import (
	"Backend/internal/services"
	"net/http"

	"github.com/labstack/echo/v4"
)

// AdminAIController は管理者向け AI / RAG 運用用の軽量コントローラです。
type AdminAIController struct {
	service *services.AdminAIService
}

func NewAdminAIController(s *services.AdminAIService) *AdminAIController {
	return &AdminAIController{service: s}
}

// Summary は RAG / AI の基本メトリクスを返します。
func (c *AdminAIController) Summary(ctx echo.Context) error {
	resp, err := c.service.GetSummary(ctx.Request().Context())
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, resp)
}

// Reembed はコレクションの再埋め込み（トリガー）を行います。
func (c *AdminAIController) Reembed(ctx echo.Context) error {
	// 今は非同期トリガーの雛形のみ
	if err := c.service.TriggerReembed(ctx.Request().Context()); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusAccepted, map[string]string{"status": "reembed triggered"})
}

// ForceResync は強制的に再調査（高価な WebSearch / LLM 呼び出し）を行うエンドポイント。
func (c *AdminAIController) ForceResync(ctx echo.Context) error {
	if err := c.service.ForceResync(ctx.Request().Context()); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusAccepted, map[string]string{"status": "resync triggered"})
}
