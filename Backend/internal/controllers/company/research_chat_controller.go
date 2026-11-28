package company

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/services/company"
	"Backend/internal/services/piimask"
	"Backend/internal/usagectx"

	"github.com/labstack/echo/v4"
)

// ResearchChatController は企業研究AIチャットの入口（SOCAIAGENT-724）。
type ResearchChatController struct {
	svc      *company.ResearchChatService
	subjects piimask.SubjectReader
}

func NewResearchChatController(svc *company.ResearchChatService, subjects piimask.SubjectReader) *ResearchChatController {
	return &ResearchChatController{svc: svc, subjects: subjects}
}

const maxResearchQuestionRunes = 500

type researchChatRequest struct {
	Question    string `json:"question"`
	CompareWith uint   `json:"compare_with"`
}

// Ask POST /api/companies/:id/research-chat
//
// 登録情報・有価証券報告書などの材料だけで答える。答えられなければ status=not_found（200）、
// 回答の生成に失敗したら 503 を返し、「情報が無い」と「失敗」を画面で区別できるようにする。
func (ctrl *ResearchChatController) Ask(ctx echo.Context) error {
	companyID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil || companyID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "ログインが必要です")
	}
	var req researchChatRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" || utf8.RuneCountInString(question) > maxResearchQuestionRunes {
		return echo.NewHTTPError(http.StatusBadRequest, "質問は1〜500文字で入力してください")
	}

	// 質問は外部の AI（ベクトル化と回答）へ送るので、先に伏字にする。伏字を作れなければ送らない（fail-closed）。
	masked, err := ctrl.mask(userID, question)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "いまは質問を受け付けられません。時間をおいて再度お試しください。")
	}

	ans, err := ctrl.svc.Ask(ctx.Request().Context(), userID, uint(companyID), req.CompareWith, masked)
	switch {
	case err == nil:
		return ctx.JSON(http.StatusOK, ans)
	case errors.Is(err, company.ErrResearchChatDisabled), errors.Is(err, company.ErrResearchChatNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "Not found")
	case errors.Is(err, company.ErrResearchChatDailyLimit):
		return echo.NewHTTPError(http.StatusTooManyRequests, "今日の質問回数の上限に達しました。明日また利用できます。")
	case errors.Is(err, usagectx.ErrAIBudgetUnavailable):
		return httpapi.AIBudgetUnavailable()
	case errors.Is(err, usagectx.ErrAIBudgetExceeded):
		return httpapi.AIBudgetExceeded(ctx)
	case errors.Is(err, company.ErrResearchChatGeneration):
		return echo.NewHTTPError(http.StatusServiceUnavailable, "回答を作れませんでした。時間をおいて再度お試しください。")
	default:
		return httpapi.InternalError(err)
	}
}

func (ctrl *ResearchChatController) mask(userID uint, text string) (string, error) {
	var subj piimask.Subject
	if ctrl.subjects != nil {
		got, err := ctrl.subjects.SubjectFor(userID)
		if err != nil {
			return "", err
		}
		subj = got
	}
	m, err := piimask.ForLLM(subj)
	if err != nil {
		return "", err
	}
	return m.Mask(text), nil
}
