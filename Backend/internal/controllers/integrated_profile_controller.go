package controllers

import (
	ifaces "Backend/internal/services/interfaces"
	"net/http"

	"github.com/labstack/echo/v4"
)

// IntegratedProfileController ユーザー統合プロファイルAPI
type IntegratedProfileController struct {
	crossFeature         ifaces.CrossFeatureIntegrationService
	interviewSessionRepo ifaces.InterviewSessionCounter
	resumeRepo           ifaces.ResumeDocumentFinder
}

func NewIntegratedProfileController(
	crossFeature ifaces.CrossFeatureIntegrationService,
	interviewSessionRepo ifaces.InterviewSessionCounter,
	resumeRepo ifaces.ResumeDocumentFinder,
) *IntegratedProfileController {
	return &IntegratedProfileController{
		crossFeature:         crossFeature,
		interviewSessionRepo: interviewSessionRepo,
		resumeRepo:           resumeRepo,
	}
}

// GetProfile GET /api/user/profile?session_id=xxx
// user_idは認証済みユーザーID(JWT検証済み、EchoUserAuth経由)を使う。
// 以前はクエリのuser_idを未検証で信頼しており、認証なしで任意ユーザーの
// プロファイル(面接回数・職務経歴書レビュー状況等)を取得できた。
func (c *IntegratedProfileController) GetProfile(ctx echo.Context) error {
	userID, ok := echoUserID(ctx)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
	}

	sessionID := ctx.QueryParam("session_id")
	if sessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "session_id is required")
	}

	// 面接セッション数を取得
	interviewCount := 0
	if count, err := c.interviewSessionRepo.CountByUser(userID); err == nil {
		interviewCount = int(count)
	}

	// 職務経歴書レビュー完了有無を確認
	resumeReviewDone := false
	if docs, err := c.resumeRepo.FindDocumentsByUserID(userID); err == nil {
		for _, doc := range docs {
			if doc.Status == "reviewed" {
				resumeReviewDone = true
				break
			}
		}
	}

	profile, err := c.crossFeature.BuildIntegratedProfile(userID, sessionID, interviewCount, resumeReviewDone)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, profile)
}
