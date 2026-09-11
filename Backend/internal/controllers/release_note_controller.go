package controllers

import (
	"Backend/internal/repositories"
	"Backend/internal/services"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type ReleaseNoteController struct {
	service  *services.ReleaseNoteService
	userRepo *repositories.UserRepository
}

func NewReleaseNoteController(service *services.ReleaseNoteService, userRepo *repositories.UserRepository) *ReleaseNoteController {
	return &ReleaseNoteController{service: service, userRepo: userRepo}
}

type releaseNoteResponse struct {
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	MergedAt time.Time `json:"merged_at"`
}

// List GET /api/whats-new
func (c *ReleaseNoteController) List(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}
	user, err := c.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	notes, err := c.service.List(ctx.Request().Context(), 20, user.Role, user.IsAdmin)
	if err != nil {
		return echoInternalError(err)
	}
	res := make([]releaseNoteResponse, 0, len(notes))
	for _, n := range notes {
		res = append(res, releaseNoteResponse{Title: n.Title, Summary: n.Summary, MergedAt: n.MergedAt})
	}
	return ctx.JSON(http.StatusOK, res)
}

type ingestSourceRequest struct {
	PRNumber uint      `json:"pr_number"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	MergedAt time.Time `json:"merged_at"`
}

type ingestRequest struct {
	Sources []ingestSourceRequest `json:"sources"`
}

// Ingest POST /api/admin/whats-new/ingest
// CIから直近マージされたPR一覧を受け取り、未取り込み分のみAI要約してDBへ保存する。
func (c *ReleaseNoteController) Ingest(ctx echo.Context) error {
	var req ingestRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	sources := make([]services.ReleaseNoteSource, 0, len(req.Sources))
	for _, s := range req.Sources {
		sources = append(sources, services.ReleaseNoteSource{
			PRNumber: s.PRNumber,
			Title:    s.Title,
			Body:     s.Body,
			MergedAt: s.MergedAt,
		})
	}

	saved, err := c.service.IngestMergedPRs(ctx.Request().Context(), sources)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]int{"saved": saved})
}
