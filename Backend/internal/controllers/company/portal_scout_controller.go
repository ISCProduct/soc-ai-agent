package company

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

type portalScoutService interface {
	ListTemplates(companyID uint) ([]models.ScoutTemplate, error)
	CreateTemplate(companyID, companyUserID uint, in companyportal.ScoutTemplateInput) (*models.ScoutTemplate, error)
	UpdateTemplate(companyID, id uint, in companyportal.ScoutTemplateInput) (*models.ScoutTemplate, error)
	DeleteTemplate(companyID, id uint) error
	Send(companyID, companyUserID uint, in companyportal.SendScoutInput) (*companyportal.SendScoutResult, error)
	ListCompanyScouts(companyID uint, limit, offset int) ([]companyportal.CompanyScoutView, int64, error)
	CooldownRemaining(companyID, userID uint) (time.Duration, error)
}

// CompanyPortalScoutController は企業ポータルのスカウト送信・テンプレート管理（#1095）。
type CompanyPortalScoutController struct {
	scouts portalScoutService
}

func NewCompanyPortalScoutController(scouts portalScoutService) *CompanyPortalScoutController {
	return &CompanyPortalScoutController{scouts: scouts}
}

type scoutTemplateBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type scoutTemplateResponse struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toScoutTemplateResponse(t *models.ScoutTemplate) scoutTemplateResponse {
	return scoutTemplateResponse{
		ID:        t.ID,
		Title:     t.Title,
		Body:      t.Body,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

type scoutResponse struct {
	ID          uint   `json:"id"`
	UserID      uint   `json:"user_id"`
	StudentName string `json:"student_name,omitempty"`
	TemplateID  *uint  `json:"template_id,omitempty"`
	Message     string `json:"message"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

func toScoutResponse(s *models.Scout) scoutResponse {
	return scoutResponse{
		ID:         s.ID,
		UserID:     s.UserID,
		TemplateID: s.TemplateID,
		Message:    s.Message,
		Status:     s.Status,
		CreatedAt:  s.CreatedAt.Format(time.RFC3339),
	}
}

func toCompanyScoutResponse(v companyportal.CompanyScoutView) scoutResponse {
	r := toScoutResponse(&v.Scout)
	r.StudentName = v.StudentName
	return r
}

// ListTemplates GET /api/company-portal/scout-templates
func (c *CompanyPortalScoutController) ListTemplates(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	items, err := c.scouts.ListTemplates(companyID)
	if err != nil {
		return mapScoutError(err)
	}
	out := make([]scoutTemplateResponse, len(items))
	for i := range items {
		out[i] = toScoutTemplateResponse(&items[i])
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// CreateTemplate POST /api/company-portal/scout-templates
func (c *CompanyPortalScoutController) CreateTemplate(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	companyUserID, ok := httpapi.CompanyUserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body scoutTemplateBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	t, err := c.scouts.CreateTemplate(companyID, companyUserID, companyportal.ScoutTemplateInput{
		Title: body.Title,
		Body:  body.Body,
	})
	if err != nil {
		return mapScoutError(err)
	}
	return ctx.JSON(http.StatusCreated, toScoutTemplateResponse(t))
}

// UpdateTemplate PATCH /api/company-portal/scout-templates/:id
func (c *CompanyPortalScoutController) UpdateTemplate(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	var body scoutTemplateBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	t, err := c.scouts.UpdateTemplate(companyID, id, companyportal.ScoutTemplateInput{
		Title: body.Title,
		Body:  body.Body,
	})
	if err != nil {
		return mapScoutError(err)
	}
	return ctx.JSON(http.StatusOK, toScoutTemplateResponse(t))
}

// DeleteTemplate DELETE /api/company-portal/scout-templates/:id
func (c *CompanyPortalScoutController) DeleteTemplate(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	id, apiErr := httpapi.UintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}
	if err := c.scouts.DeleteTemplate(companyID, id); err != nil {
		return mapScoutError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

type sendScoutBody struct {
	UserID     uint   `json:"user_id"`
	TemplateID uint   `json:"template_id"`
	Message    string `json:"message"`
}

// Send POST /api/company-portal/scouts
func (c *CompanyPortalScoutController) Send(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	companyUserID, ok := httpapi.CompanyUserID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	var body sendScoutBody
	if err := ctx.Bind(&body); err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "Invalid request body")
	}
	res, err := c.scouts.Send(companyID, companyUserID, companyportal.SendScoutInput{
		UserID:     body.UserID,
		TemplateID: body.TemplateID,
		Message:    body.Message,
	})
	if err != nil {
		if errors.Is(err, companyportal.ErrScoutCooldown) {
			remaining := int64(0)
			if res != nil {
				remaining = res.RemainingMs
			}
			return ctx.JSON(http.StatusTooManyRequests, map[string]any{
				"code":           httpapi.ErrCodeTooManyRequests,
				"message":        "同じ学生への再送は24時間空ける必要があります",
				"remaining_ms":   remaining,
				"cooldown_hours": 24,
			})
		}
		return mapScoutError(err)
	}
	return ctx.JSON(http.StatusCreated, toScoutResponse(res.Scout))
}

// List GET /api/company-portal/scouts
func (c *CompanyPortalScoutController) List(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	items, total, err := c.scouts.ListCompanyScouts(
		companyID,
		httpapi.LimitQuery(ctx, "limit", 30),
		httpapi.IntQuery(ctx, "offset", 0),
	)
	if err != nil {
		return mapScoutError(err)
	}
	out := make([]scoutResponse, len(items))
	for i := range items {
		out[i] = toCompanyScoutResponse(items[i])
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "total": total})
}

// Cooldown GET /api/company-portal/scouts/cooldown?user_id=
func (c *CompanyPortalScoutController) Cooldown(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return httpapi.NewAPIError(http.StatusUnauthorized, httpapi.ErrCodeUnauthorized, "Unauthorized")
	}
	uid, err := httpapi.RequiredUintQuery(ctx, "user_id")
	if err != nil {
		return httpapi.NewAPIError(http.StatusBadRequest, httpapi.ErrCodeValidationError, "user_id は必須です")
	}
	remaining, err := c.scouts.CooldownRemaining(companyID, uid)
	if err != nil {
		return mapScoutError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"user_id":      uid,
		"remaining_ms": remaining.Milliseconds(),
		"blocked":      remaining > 0,
	})
}

func mapScoutError(err error) error {
	var ve *shared.ValidationError
	switch {
	case errors.Is(err, shared.ErrForbidden):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "権限がありません")
	case errors.Is(err, companyportal.ErrScoutNotFound), errors.Is(err, shared.ErrNotFound):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "対象が見つかりません")
	case errors.Is(err, companyportal.ErrScoutNotVisible):
		return httpapi.NewAPIError(http.StatusNotFound, httpapi.ErrCodeNotFound, "学生が見つかりません（公開されていない可能性があります）")
	case errors.Is(err, companyportal.ErrScoutBlocked):
		return httpapi.NewAPIError(http.StatusForbidden, httpapi.ErrCodeForbidden, "学生がこの企業をブロックしているため送れません")
	case errors.As(err, &ve):
		return httpapi.NewAPIError(http.StatusUnprocessableEntity, httpapi.ErrCodeValidationError, ve.Message)
	default:
		return httpapi.InternalError(err)
	}
}
