package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"Backend/internal/services/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// AdminInterviewController provides admin endpoints for viewing interview sessions and videos.
type AdminInterviewController struct {
	interviewService    *services.InterviewService
	videoRepo           repository.InterviewVideoRepository
	s3Service           *storage.S3UploadService
	companyQuestionRepo repository.InterviewCompanyQuestionRepository
	companyRepo         repository.CompanyRepository
	openaiClient        *openai.Client
	access              services.UserAccessGuard
}

func NewAdminInterviewController(
	interviewService *services.InterviewService,
	videoRepo repository.InterviewVideoRepository,
	s3Service *storage.S3UploadService,
) *AdminInterviewController {
	return &AdminInterviewController{
		interviewService: interviewService,
		videoRepo:        videoRepo,
		s3Service:        s3Service,
	}
}

// SetCompanyQuestionRepo 企業別質問リポジトリを注入する
func (c *AdminInterviewController) SetCompanyQuestionRepo(r repository.InterviewCompanyQuestionRepository) {
	c.companyQuestionRepo = r
}

// SetCompanyRepo 企業リポジトリを注入する
func (c *AdminInterviewController) SetCompanyRepo(r repository.CompanyRepository) {
	c.companyRepo = r
}

// SetOpenAIClient OpenAIクライアントを注入する
func (c *AdminInterviewController) SetOpenAIClient(client *openai.Client) {
	c.openaiClient = client
}

// SetUserAccessGuard 退会済みユーザーの動画閲覧を遮断するために注入する
func (c *AdminInterviewController) SetUserAccessGuard(guard services.UserAccessGuard) {
	c.access = guard
}

// ListCompanyQuestions GET /api/admin/companies/:id/interview-questions
func (c *AdminInterviewController) ListCompanyQuestions(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qs, err := c.companyQuestionRepo.FindByCompanyID(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"questions": qs})
}

// CreateCompanyQuestion POST /api/admin/companies/:id/interview-questions
func (c *AdminInterviewController) CreateCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	var req struct {
		Position     string `json:"position"`
		Category     string `json:"category"`
		QuestionText string `json:"question_text"`
		Priority     int    `json:"priority"`
		IsRequired   bool   `json:"is_required"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	req.Position = strings.TrimSpace(req.Position)
	req.Category = strings.TrimSpace(req.Category)
	req.QuestionText = strings.TrimSpace(req.QuestionText)
	if req.Category == "" || req.QuestionText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "category and question_text are required")
	}
	q := &models.InterviewCompanyQuestion{
		CompanyID:    companyID,
		Position:     req.Position,
		Category:     req.Category,
		QuestionText: req.QuestionText,
		Priority:     req.Priority,
		IsRequired:   req.IsRequired,
	}
	if err := c.companyQuestionRepo.Create(q); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusCreated, q)
}

// UpdateCompanyQuestion PUT /api/admin/companies/:id/interview-questions/:qid
func (c *AdminInterviewController) UpdateCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qID, err := echoUintParam(ctx, "qid")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	q, err := c.companyQuestionRepo.FindByID(qID)
	if err != nil || q == nil || q.CompanyID != companyID {
		return echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}
	var req struct {
		Position     *string `json:"position"`
		Category     *string `json:"category"`
		QuestionText *string `json:"question_text"`
		Priority     *int    `json:"priority"`
		IsRequired   *bool   `json:"is_required"`
	}
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.Position != nil {
		q.Position = strings.TrimSpace(*req.Position)
	}
	if req.Category != nil {
		category := strings.TrimSpace(*req.Category)
		if category == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "category must not be empty")
		}
		q.Category = category
	}
	if req.QuestionText != nil {
		questionText := strings.TrimSpace(*req.QuestionText)
		if questionText == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "question_text must not be empty")
		}
		q.QuestionText = questionText
	}
	if req.Priority != nil {
		q.Priority = *req.Priority
	}
	if req.IsRequired != nil {
		q.IsRequired = *req.IsRequired
	}
	if err := c.companyQuestionRepo.Update(q); err != nil {
		return echoInternalError(err)
	}
	return ctx.JSON(http.StatusOK, q)
}

// DeleteCompanyQuestion DELETE /api/admin/companies/:id/interview-questions/:qid
func (c *AdminInterviewController) DeleteCompanyQuestion(ctx echo.Context) error {
	if c.companyQuestionRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	qID, err := echoUintParam(ctx, "qid")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid question ID")
	}
	q, err := c.companyQuestionRepo.FindByID(qID)
	if err != nil || q == nil || q.CompanyID != companyID {
		return echo.NewHTTPError(http.StatusNotFound, "Question not found")
	}
	if err := c.companyQuestionRepo.Delete(qID); err != nil {
		return echoInternalError(err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

// GenerateCompanyQuestions POST /api/admin/companies/:id/interview-questions/generate
// 企業情報・求人情報をもとにAIが面接質問候補を生成して返す（DBには保存しない）。
func (c *AdminInterviewController) GenerateCompanyQuestions(ctx echo.Context) error {
	if c.companyRepo == nil || c.openaiClient == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "not configured")
	}
	companyID, err := echoUintParam(ctx, "id")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	company, err := c.companyRepo.FindByID(companyID)
	if err != nil || company == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Company not found")
	}

	positions, _ := c.companyRepo.FindJobPositionsByCompany(companyID)

	reqCtx, cancel := context.WithTimeout(ctx.Request().Context(), 60*time.Second)
	defer cancel()

	questions, err := c.generateQuestionsWithAI(reqCtx, company, positions)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("AI生成失敗: %v", err))
	}

	// DBには保存せず候補として返す
	return ctx.JSON(http.StatusOK, map[string]any{
		"questions": questions,
		"total":     len(questions),
	})
}

// GeneratedQuestion はAI生成の質問候補（IDなし）。
type GeneratedQuestion struct {
	Category     string `json:"category"`
	Position     string `json:"position"`
	QuestionText string `json:"question_text"`
	Priority     int    `json:"priority"`
	IsRequired   bool   `json:"is_required"`
}

// generateQuestionsWithAI は企業情報をもとにAIで面接質問を生成する。
func (c *AdminInterviewController) generateQuestionsWithAI(ctx context.Context, company *models.Company, positions []models.CompanyJobPosition) ([]*GeneratedQuestion, error) {
	positionSummary := buildPositionSummaryForInterview(positions)

	systemPrompt := `あなたは採用面接の専門家です。企業情報をもとに、その企業の面接で聞かれそうな質問を生成してください。
以下のJSON配列形式のみで回答してください（説明文は不要）。`

	userPrompt := fmt.Sprintf(`企業名: %s
業種: %s
企業文化: %s
主要事業: %s
技術スタック: %s
求人情報: %s

この企業の面接で聞かれそうな質問を10〜15件生成してください。
カテゴリは「志望動機」「職務経験」「技術」「カルチャーフィット」「強み・弱み」「キャリアビジョン」「その他」から選んでください。
JSON配列形式のみで回答（説明文不要）:
[
  {
    "category": "カテゴリ名",
    "position": "対象職種（全職種共通なら空文字）",
    "question_text": "質問文",
    "priority": 優先度（0〜9の整数、小さいほど重要）,
    "is_required": true or false
  }
]`,
		company.Name,
		company.Industry,
		company.Culture,
		company.MainBusiness,
		company.TechStack,
		positionSummary,
	)

	jsonStr, err := c.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.7, 2000)
	if err != nil {
		return nil, err
	}

	start := strings.Index(jsonStr, "[")
	end := strings.LastIndex(jsonStr, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("JSONの配列が見つかりません")
	}

	var raw []struct {
		Category     string `json:"category"`
		Position     string `json:"position"`
		QuestionText string `json:"question_text"`
		Priority     int    `json:"priority"`
		IsRequired   bool   `json:"is_required"`
	}
	if err := json.Unmarshal([]byte(jsonStr[start:end+1]), &raw); err != nil {
		return nil, fmt.Errorf("JSONパース失敗: %w", err)
	}

	validCategories := map[string]bool{
		"志望動機": true, "職務経験": true, "技術": true,
		"カルチャーフィット": true, "強み・弱み": true, "キャリアビジョン": true, "その他": true,
	}

	result := make([]*GeneratedQuestion, 0, len(raw))
	for _, r := range raw {
		cat := strings.TrimSpace(r.Category)
		if !validCategories[cat] {
			cat = "その他"
		}
		qt := strings.TrimSpace(r.QuestionText)
		if qt == "" {
			continue
		}
		result = append(result, &GeneratedQuestion{
			Category:     cat,
			Position:     strings.TrimSpace(r.Position),
			QuestionText: qt,
			Priority:     r.Priority,
			IsRequired:   r.IsRequired,
		})
	}
	return result, nil
}

func buildPositionSummaryForInterview(positions []models.CompanyJobPosition) string {
	if len(positions) == 0 {
		return "（求人情報なし）"
	}
	var sb strings.Builder
	for _, p := range positions {
		fmt.Fprintf(&sb, "・%s（%s）\n", p.Title, p.EmploymentType)
	}
	return sb.String()
}

// ListSessions handles GET /api/admin/interviews
// Returns all interview sessions with pagination.
func (c *AdminInterviewController) ListSessions(ctx echo.Context) error {
	page := echoIntQuery(ctx, "page", 1)
	limit := echoIntQuery(ctx, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	sessions, total, err := c.interviewService.ListAllSessionsAdmin(limit, offset)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"sessions": sessions,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// ListVideos handles GET /api/admin/interviews/:id/videos
func (c *AdminInterviewController) ListVideos(ctx echo.Context) error {
	sessionID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid session ID")
	}

	videos, err := c.videoRepo.FindBySessionID(ctx.Request().Context(), uint(sessionID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch videos")
	}

	return ctx.JSON(http.StatusOK, map[string]any{
		"videos": videos,
	})
}

// VideoURL handles GET /api/admin/interviews/:id/videos/:video_id/url
// Returns a presigned S3 URL valid for 15 minutes.
func (c *AdminInterviewController) VideoURL(ctx echo.Context) error {
	videoID, err := strconv.ParseUint(ctx.Param("video_id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid video ID")
	}

	video, err := c.videoRepo.FindByID(ctx.Request().Context(), uint(videoID))
	if err != nil || video == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Video not found")
	}

	if video.Status != "done" || video.DriveFileID == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "Video is not available yet")
	}

	if c.access != nil {
		if err := c.access.EnsureActiveUser(video.UserID); err != nil {
			if errors.Is(err, services.ErrAccountWithdrawn) {
				return echo.NewHTTPError(http.StatusForbidden, "account has been withdrawn")
			}
			return echoInternalError(err)
		}
	}

	if c.s3Service == nil {
		// S3 not configured: return the stored URL directly
		return ctx.JSON(http.StatusOK, map[string]string{
			"url":        video.DriveFileURL,
			"expires_at": "",
		})
	}

	expires := 15 * time.Minute
	presignedURL, err := c.s3Service.PresignGetURL(ctx.Request().Context(), video.DriveFileID, expires)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, map[string]string{
		"url":        presignedURL,
		"expires_at": time.Now().Add(expires).UTC().Format(time.RFC3339),
	})
}
