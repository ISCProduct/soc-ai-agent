package assessment

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	assessmentsvc "Backend/internal/services/assessment"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Controller は学生の適性検査 API（#1678）。認証は EchoUserAuth に委ねる。
type Controller struct {
	svc *assessmentsvc.Service
}

func NewController(svc *assessmentsvc.Service) *Controller {
	return &Controller{svc: svc}
}

// List GET /api/assessments
func (c *Controller) List(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	rows, err := c.svc.List(userID)
	if err != nil {
		return httpapi.InternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"assessments": rows})
}

// Start POST /api/assessments/:templateId/sessions
func (c *Controller) Start(ctx echo.Context) error {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	id, err := strconv.ParseUint(ctx.Param("templateId"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "検査の指定が正しくありません")
	}
	v, err := c.svc.Start(userID, uint(id))
	if err != nil {
		return mapError(err)
	}
	return ctx.JSON(http.StatusOK, v)
}

// Get GET /api/assessments/sessions/:sessionId
func (c *Controller) Get(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	v, err := c.svc.Get(userID, sessionID)
	if err != nil {
		return mapError(err)
	}
	return ctx.JSON(http.StatusOK, v)
}

// SaveAnswer PUT /api/assessments/sessions/:sessionId/answers/:questionId  {"value": 1..5}
func (c *Controller) SaveAnswer(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	qid, err := strconv.ParseUint(ctx.Param("questionId"), 10, 64)
	if err != nil || qid == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "問題の指定が正しくありません")
	}
	var body struct {
		Value int `json:"value"`
	}
	if err := ctx.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "回答の形式が正しくありません")
	}
	if err := c.svc.SaveAnswer(userID, sessionID, uint(qid), body.Value); err != nil {
		return mapError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// Submit POST /api/assessments/sessions/:sessionId/submit
func (c *Controller) Submit(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	if _, err := c.svc.Submit(ctx.Request().Context(), userID, sessionID); err != nil {
		return mapError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"session_id": sessionID})
}

// Result GET /api/assessments/sessions/:sessionId/result
func (c *Controller) Result(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	v, err := c.svc.Result(userID, sessionID)
	if err != nil {
		return mapError(err)
	}
	return ctx.JSON(http.StatusOK, v)
}

func sessionParams(ctx echo.Context) (uint, string, error) {
	userID, ok := httpapi.UserID(ctx)
	if !ok {
		return 0, "", echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	sessionID := ctx.Param("sessionId")
	if _, err := uuid.Parse(sessionID); err != nil {
		return 0, "", echo.NewHTTPError(http.StatusBadRequest, "受検の指定が正しくありません")
	}
	return userID, sessionID, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, assessmentsvc.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "検査が見つかりません")
	case errors.Is(err, assessmentsvc.ErrAlreadySubmitted):
		return echo.NewHTTPError(http.StatusConflict, "この検査はすでに提出済みです")
	case errors.Is(err, assessmentsvc.ErrIncomplete):
		return echo.NewHTTPError(http.StatusBadRequest, "まだ答えていない問題があります")
	case errors.Is(err, assessmentsvc.ErrInvalidAnswer):
		return echo.NewHTTPError(http.StatusBadRequest, "回答は1〜5で選んでください")
	}
	return httpapi.InternalError(err)
}
