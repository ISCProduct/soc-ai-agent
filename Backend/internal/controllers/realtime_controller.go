package controllers

import (
	"Backend/internal/openai"
	ifaces "Backend/internal/services/interfaces"
	"Backend/internal/services/shared"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type RealtimeController struct {
	interviewService     ifaces.InterviewService
	realtimeUsageService ifaces.RealtimeUsageService
}

func NewRealtimeController(interviewService ifaces.InterviewService, realtimeUsageService ifaces.RealtimeUsageService) *RealtimeController {
	return &RealtimeController{interviewService: interviewService, realtimeUsageService: realtimeUsageService}
}

type realtimeTokenRequest struct {
	UserID      uint `json:"user_id"`
	InterviewID uint `json:"interview_id"`
}

type realtimeTokenResponse struct {
	ClientSecret string `json:"client_secret"`
}

// Token POST /api/realtime/token
func (c *RealtimeController) Token(ctx echo.Context) error {
	var req realtimeTokenRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.InterviewID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "interview_id is required")
	}
	// 誰として振る舞うかはトークンだけで決める。
	// ボディの user_id を信頼すると、CreateRealtimeToken 内の isAllowed が
	// actorID == ownerID で通るため、被害者のIDとセッションIDを両方指定するだけで
	// 他人の面接セッションの ephemeral key を発行できてしまう(IDOR)。
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	// 旧クライアントは user_id を送る。食い違いはクライアント側の不具合なので明示的に弾く。
	if req.UserID != 0 && req.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	secret, err := c.interviewService.CreateRealtimeToken(ctx.Request().Context(), userID, req.InterviewID)
	if err != nil {
		if errors.Is(err, shared.ErrForbidden) {
			return echo.NewHTTPError(http.StatusForbidden, err.Error())
		}
		if strings.Contains(err.Error(), "realtime capacity exceeded") {
			return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
		}
		// AI プロバイダ未設定・ローカル構成では 503 + 固定文言にする。
		// err.Error() をそのまま返すと OpenAI のエラー本文や内部設定が学生に見える(#1293)
		if errors.Is(err, openai.ErrAIUnavailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "現在AI機能を利用できません。しばらくしてから再度お試しください。")
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return ctx.JSON(http.StatusOK, realtimeTokenResponse{ClientSecret: secret})
}

type sessionInfoResponse struct {
	SessionMinutes int `json:"session_minutes"`
}

// SessionInfo GET /api/realtime/session-info
// ユーザー向けのセッション時間（分）を返す。コスト情報は含まない。
func (c *RealtimeController) SessionInfo(ctx echo.Context) error {
	minutes := 10
	if c.realtimeUsageService != nil {
		minutes = c.realtimeUsageService.SessionDurationMinutes()
	}
	return ctx.JSON(http.StatusOK, sessionInfoResponse{SessionMinutes: minutes})
}
