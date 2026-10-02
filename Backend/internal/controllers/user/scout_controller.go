package user

import (
	"errors"
	"net/http"
	"time"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/models"
	"Backend/internal/services/companyportal"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

type studentScoutService interface {
	ListStudentScouts(userID uint, limit, offset int) ([]companyportal.StudentScoutView, int64, error)
	MarkViewed(userID, scoutID uint) (*models.Scout, error)
	Decline(userID, scoutID uint) (*models.Scout, error)
	BlockCompany(userID, companyID uint) error
	ListBlockedCompanyIDs(userID uint) ([]uint, error)
}

// StudentScoutController は学生側のスカウト受信・辞退・ブロック（#1095）。
type StudentScoutController struct {
	scouts studentScoutService
}

func NewStudentScoutController(scouts studentScoutService) *StudentScoutController {
	return &StudentScoutController{scouts: scouts}
}

type studentScoutItem struct {
	ID          uint   `json:"id"`
	CompanyID   uint   `json:"company_id"`
	CompanyName string `json:"company_name"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

func toStudentScoutItem(v companyportal.StudentScoutView) studentScoutItem {
	return studentScoutItem{
		ID:          v.ID,
		CompanyID:   v.CompanyID,
		CompanyName: v.CompanyName,
		Message:     v.Message,
		Status:      v.Status,
		CreatedAt:   v.CreatedAt.Format(time.RFC3339),
	}
}

// List GET /api/user/scouts
func (c *StudentScoutController) List(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	items, total, err := c.scouts.ListStudentScouts(
		userID,
		httpapi.LimitQuery(ctx, "limit", 30),
		httpapi.IntQuery(ctx, "offset", 0),
	)
	if err != nil {
		return mapStudentScoutError(err)
	}
	out := make([]studentScoutItem, len(items))
	for i := range items {
		out[i] = toStudentScoutItem(items[i])
	}
	blocked, err := c.scouts.ListBlockedCompanyIDs(userID)
	if err != nil {
		return mapStudentScoutError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"items":                  out,
		"total":                  total,
		"blocked_company_ids":    blocked,
	})
}

// View POST /api/user/scouts/:id/view
func (c *StudentScoutController) View(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	scout, err := c.scouts.MarkViewed(userID, id)
	if err != nil {
		return mapStudentScoutError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"id":         scout.ID,
		"status":     scout.Status,
		"message":    scout.Message,
		"company_id": scout.CompanyID,
		"created_at": scout.CreatedAt.Format(time.RFC3339),
	})
}

// Decline POST /api/user/scouts/:id/decline
func (c *StudentScoutController) Decline(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	scout, err := c.scouts.Decline(userID, id)
	if err != nil {
		return mapStudentScoutError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"id":     scout.ID,
		"status": scout.Status,
	})
}

type blockCompanyBody struct {
	CompanyID uint `json:"company_id"`
}

// BlockCompany POST /api/user/scout-blocks
func (c *StudentScoutController) BlockCompany(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body blockCompanyBody
	if err := ctx.Bind(&body); err != nil || body.CompanyID == 0 {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "company_id は必須です")
	}
	if err := c.scouts.BlockCompany(userID, body.CompanyID); err != nil {
		return mapStudentScoutError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

func mapStudentScoutError(err error) error {
	var ve *shared.ValidationError
	switch {
	case errors.Is(err, companyportal.ErrScoutNotFound), errors.Is(err, shared.ErrNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "スカウトが見つかりません")
	case errors.Is(err, shared.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "権限がありません")
	case errors.As(err, &ve):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, httpapi.ErrCodeValidationError, ve.Message)
	default:
		return httpapi.InternalError(err)
	}
}
