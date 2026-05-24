package controllers

import (
	ifaces "Backend/internal/services/interfaces"
	"net/http"
	"strconv"

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

// GetProfile GET /api/user/profile?user_id=xxx&session_id=xxx
func (c *IntegratedProfileController) GetProfile(ctx echo.Context) error {
	userIDStr := ctx.QueryParam("user_id")
	if userIDStr == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	userIDParsed, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user_id")
	}
	userID := uint(userIDParsed)

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
