package user

import (
	"Backend/internal/controllers/httpapi"
	"Backend/internal/services/costs"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AIBudgetController struct {
	service *costs.AIBudgetService
}

func NewAIBudgetController(service *costs.AIBudgetService) *AIBudgetController {
	return &AIBudgetController{service: service}
}

// Me は GET /api/user/ai-budget の利用者別・学校別残量を返す。
func (c *AIBudgetController) Me(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	status, err := c.service.Status(ctx.Request().Context(), userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, status)
}
