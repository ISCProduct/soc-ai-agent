// Package ability は学生の能力検査 API（SOCAIAGENT-433）。認証は EchoUserAuth に委ねる。
package ability

import (
	"errors"
	"net/http"
	"strconv"

	"Backend/internal/controllers/httpapi"
	abilitysvc "Backend/internal/services/ability"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Controller struct {
	svc *abilitysvc.Service
}

func NewController(svc *abilitysvc.Service) *Controller {
	return &Controller{svc: svc}
}

// List GET /api/ability-assessments
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

// Start POST /api/ability-assessments/:templateId/sessions
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

// Get GET /api/ability-assessments/sessions/:sessionId
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

// SaveAnswer PUT /api/ability-assessments/sessions/:sessionId/answers/:questionId  {"choice": 0..n-1}
func (c *Controller) SaveAnswer(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	qid, err := strconv.ParseUint(ctx.Param("questionId"), 10, 64)
	if err != nil || qid == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "問題の指定が正しくありません")
	}
	// choice は 0 を正当な値として受けるので、未指定と 0 を区別するためポインタで受ける。
	var body struct {
		Choice *int `json:"choice"`
	}
	if err := ctx.Bind(&body); err != nil || body.Choice == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "回答の形式が正しくありません")
	}
	if err := c.svc.SaveAnswer(userID, sessionID, uint(qid), *body.Choice); err != nil {
		return mapError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// Submit POST /api/ability-assessments/sessions/:sessionId/submit
func (c *Controller) Submit(ctx echo.Context) error {
	userID, sessionID, err := sessionParams(ctx)
	if err != nil {
		return err
	}
	if _, err := c.svc.Submit(userID, sessionID); err != nil {
		return mapError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"session_id": sessionID})
}

// Result GET /api/ability-assessments/sessions/:sessionId/result
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
	case errors.Is(err, abilitysvc.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "検査が見つかりません")
	case errors.Is(err, abilitysvc.ErrAlreadySubmitted):
		return echo.NewHTTPError(http.StatusConflict, "この検査はすでに提出済みです")
	// 410: 制限時間を過ぎた。再開も追加回答もできないことを、404 や 409 と区別して伝える。
	case errors.Is(err, abilitysvc.ErrDeadlinePassed):
		return echo.NewHTTPError(http.StatusGone, "制限時間を過ぎました。これまでの回答で提出できます")
	case errors.Is(err, abilitysvc.ErrInvalidChoice):
		return echo.NewHTTPError(http.StatusBadRequest, "選択肢から選んでください")
	}
	return httpapi.InternalError(err)
}
