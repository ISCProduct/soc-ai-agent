package admin

import (
	"Backend/domain/repository"
	"Backend/internal/controllers/httpapi"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// diagnosisQualityReader は診断品質レポートの読み取り口。
// 一覧の limit がリポジトリへ渡る値をテストで固定するため、具象型ではなくこの形に依存する。
type diagnosisQualityReader interface {
	FindByUserAndSession(userID uint, sessionID string) (*models.DiagnosisQualityReport, error)
	ListRecent(limit int) ([]models.DiagnosisQualityReport, error)
}

// AdminDiagnosisQualityController は診断妥当性レポートの参照API。
type AdminDiagnosisQualityController struct {
	repo diagnosisQualityReader
}

func NewAdminDiagnosisQualityController(repo *repositories.DiagnosisQualityRepository) *AdminDiagnosisQualityController {
	return &AdminDiagnosisQualityController{repo: repo}
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
	report, err := c.repo.FindByUserAndSession(uint(userID64), sessionID)
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
	// 他の管理系一覧と同じ 100 で頭打ちにする。未指定は 50。
	// 上限が無いと limit=1000000 がそのまま一覧取得へ渡る（SOCAIAGENT-530）。
	limit := httpapi.LimitQuery(ctx, "limit", 50)
	rows, err := c.repo.ListRecent(limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": rows})
}
