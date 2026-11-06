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

// aiBudgetResponse は利用者に見せる予算の状態。
// 学校の上限と利用額は運用情報なので、止まっているかどうかだけを返す。
type aiBudgetResponse struct {
	Date          string                    `json:"date"`
	User          costs.AIBudgetScopeStatus `json:"user"`
	SchoolBlocked bool                      `json:"school_blocked"`
	Allowed       bool                      `json:"allowed"`
}

// Me は GET /api/user/ai-budget の利用者の残量と、学校の上限で止まっているかを返す。
func (c *AIBudgetController) Me(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	status, err := c.service.Status(ctx.Request().Context(), userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	schoolBlocked := status.School != nil && (status.School.Daily.Exceeded || status.School.Monthly.Exceeded)
	return ctx.JSON(http.StatusOK, aiBudgetResponse{
		Date:          status.Date,
		User:          status.User,
		SchoolBlocked: schoolBlocked,
		Allowed:       status.Allowed,
	})
}
