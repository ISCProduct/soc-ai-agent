package admin

import (
	"Backend/domain/repository"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/diagnosis"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// AdminDiagnosisQualityController は診断妥当性レポートの参照API。
type AdminDiagnosisQualityController struct {
	service *diagnosis.QualityService
}

func NewAdminDiagnosisQualityController(service *diagnosis.QualityService) *AdminDiagnosisQualityController {
	return &AdminDiagnosisQualityController{service: service}
}

// GetBySession GET /api/admin/diagnosis-quality?user_id=&session_id=
func (c *AdminDiagnosisQualityController) GetBySession(ctx echo.Context) error {
	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "session_id is required"})
	}
	userID64, err := strconv.ParseUint(ctx.QueryParam("user_id"), 10, 64)
	if err != nil || userID64 == 0 {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "user_id is required"})
	}
	report, err := c.service.FindReport(uint(userID64), sessionID)
	if err != nil {
		if err == repository.ErrNotFound {
			return ctx.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, report)
}

// List GET /api/admin/diagnosis-quality?limit=
func (c *AdminDiagnosisQualityController) List(ctx echo.Context) error {
	if ctx.QueryParam("session_id") != "" {
		return c.GetBySession(ctx)
	}
	// 他の管理系一覧と同じ 100 で頭打ちにする。未指定・0・数値以外は 50。
	// 上限が無いと limit=1000000 がそのまま一覧取得へ渡る（SOCAIAGENT-530）。
	limit := httpapi.LimitQuery(ctx, "limit", 50)
	rows, err := c.service.ListRecentReports(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": rows})
}
