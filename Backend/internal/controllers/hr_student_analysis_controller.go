package controllers

import (
	hrsvc "Backend/internal/services/hr"
	"Backend/internal/services/shared"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HRStudentAnalysisController 企業向け学生分析プロファイルAPI（#1096）
type HRStudentAnalysisController struct {
	svc studentAnalysisService
}

type studentAnalysisService interface {
	GetAnalysis(ownerUserID, companyID, targetUserID uint) (*hrsvc.StudentAnalysisResponse, error)
}

func NewHRStudentAnalysisController(svc studentAnalysisService) *HRStudentAnalysisController {
	return &HRStudentAnalysisController{svc: svc}
}

// GetAnalysis GET /api/hr/students/:userID/analysis?company_id=
func (c *HRStudentAnalysisController) GetAnalysis(ctx echo.Context) error {
	ownerUserID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "認証が必要です")
	}
	companyID, err := echoRequiredUintQuery(ctx, "company_id")
	if err != nil {
		return err
	}
	targetUserID, err := echoUintParam(ctx, "userID")
	if err != nil {
		return err
	}

	analysis, err := c.svc.GetAnalysis(ownerUserID, companyID, targetUserID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, "企業の所有権がありません")
		}
		if errors.Is(err, hrsvc.ErrStudentNotVisible) {
			return echo.NewHTTPError(http.StatusNotFound, "学生が見つかりません")
		}
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, analysis)
}
