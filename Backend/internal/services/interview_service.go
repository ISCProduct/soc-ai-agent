package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type InterviewService struct {
	sessionRepo          repository.InterviewSessionRepository
	utterRepo            repository.InterviewUtteranceRepository
	reportRepo           repository.InterviewReportRepository
	userRepo             repository.UserRepository
	emailService         *EmailService
	openaiClient         *openai.Client
	realtimeUsageService *RealtimeUsageService
	crossFeature         *CrossFeatureIntegrationService
	jobCh                chan uint
	workerOnce           sync.Once
}

func NewInterviewService(
	sessionRepo repository.InterviewSessionRepository,
	utterRepo repository.InterviewUtteranceRepository,
	reportRepo repository.InterviewReportRepository,
	userRepo repository.UserRepository,
	emailService *EmailService,
	openaiClient *openai.Client,
	realtimeUsageService *RealtimeUsageService,
) *InterviewService {
	return &InterviewService{
		sessionRepo:          sessionRepo,
		utterRepo:            utterRepo,
		reportRepo:           reportRepo,
		userRepo:             userRepo,
		emailService:         emailService,
		openaiClient:         openaiClient,
		realtimeUsageService: realtimeUsageService,
		jobCh:                make(chan uint, 100),
	}
}

// SetCrossFeatureService 機能間連携サービスを注入する（オプション）
func (s *InterviewService) SetCrossFeatureService(cf *CrossFeatureIntegrationService) {
	s.crossFeature = cf
}

func (s *InterviewService) StartWorker() {
	s.workerOnce.Do(func() {
		go s.runWorker()
	})
}

func (s *InterviewService) runWorker() {
	for sessionID := range s.jobCh {
		if err := s.generateReport(context.Background(), sessionID); err != nil {
			fmt.Printf("[Interview] Report generation failed for session %d: %v\n", sessionID, err)
			continue
		}
		fmt.Printf("[Interview] Report generation completed for session %d\n", sessionID)
	}
}

type InterviewSessionResponse struct {
	ID                uint       `json:"id"`
	UserID            uint       `json:"user_id"`
	Status            string     `json:"status"`
	Language          string     `json:"language"`
	InterviewerGender string     `json:"interviewer_gender"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	EstimatedCostUSD  float64    `json:"estimated_cost_usd"`
	TemplateVersion   string     `json:"template_version"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type InterviewDetailResponse struct {
	Session    InterviewSessionResponse    `json:"session"`
	Utterances []models.InterviewUtterance `json:"utterances"`
	Report     *models.InterviewReport     `json:"report,omitempty"`
}

func (s *InterviewService) CreateSession(userID uint, language string, interviewerGender string) (*InterviewSessionResponse, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if language == "" {
		language = "ja"
	}
	if interviewerGender != "male" && interviewerGender != "female" {
		interviewerGender = "female"
	}
	session := &models.InterviewSession{
		UserID:            userID,
		Status:            "ready",
		Language:          language,
		InterviewerGender: interviewerGender,
		TemplateVersion:   getEnv("INTERVIEW_TEMPLATE_VERSION", "v1"),
	}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) StartSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	if session.Status == "finished" {
		return nil, errors.New("session already finished")
	}
	if session.StartedAt == nil {
		now := time.Now()
		session.StartedAt = &now
	}
	session.Status = "in_progress"
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	return toSessionResponse(session), nil
}

func (s *InterviewService) FinishSession(userID uint, sessionID uint) (*InterviewSessionResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	if session.EndedAt == nil {
		now := time.Now()
		session.EndedAt = &now
	}
	session.Status = "finished"
	if s.realtimeUsageService != nil {
		if durationSec, cost, err := s.realtimeUsageService.CloseSession(sessionID, *session.EndedAt); err == nil && durationSec >= 0 {
			session.EstimatedCostUSD = cost
		} else {
			session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
		}
	} else {
		session.EstimatedCostUSD = s.estimateCost(session.StartedAt, session.EndedAt)
	}
	if err := s.sessionRepo.Update(session); err != nil {
		return nil, err
	}
	s.jobCh <- sessionID
	return toSessionResponse(session), nil
}

func (s *InterviewService) SaveUtterance(userID uint, sessionID uint, role string, text string) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !s.isAllowed(userID, session.UserID) {
		return errors.New("forbidden")
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "user" && role != "ai" {
		return errors.New("invalid role")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty text")
	}
	utter := &models.InterviewUtterance{
		SessionID: sessionID,
		Role:      role,
		Text:      text,
	}
	return s.utterRepo.Create(utter)
}

// ListAllSessionsAdmin lists all interview sessions without performing a user-level admin check.
// The caller (admin middleware) is responsible for ensuring only admins can invoke this.
func (s *InterviewService) ListAllSessionsAdmin(limit int, offset int) ([]InterviewSessionResponse, int64, error) {
	total, err := s.sessionRepo.CountAll()
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListAll(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

func (s *InterviewService) ListSessions(userID uint, all bool, limit int, offset int) ([]InterviewSessionResponse, int64, error) {
	if all {
		user, err := s.userRepo.GetUserByID(userID)
		if err != nil || user == nil || !user.IsAdmin {
			return nil, 0, errors.New("forbidden")
		}
		total, err := s.sessionRepo.CountAll()
		if err != nil {
			return nil, 0, err
		}
		sessions, err := s.sessionRepo.ListAll(limit, offset)
		if err != nil {
			return nil, 0, err
		}
		return toSessionResponses(sessions), total, nil
	}
	total, err := s.sessionRepo.CountByUser(userID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := s.sessionRepo.ListByUser(userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toSessionResponses(sessions), total, nil
}

func (s *InterviewService) GetSessionDetail(userID uint, sessionID uint) (*InterviewDetailResponse, error) {
	return s.GetSessionDetailWithRole(userID, sessionID, "student")
}

func (s *InterviewService) GetSessionDetailWithRole(userID uint, sessionID uint, role string) (*InterviewDetailResponse, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	var report *models.InterviewReport
	report, err = s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		report = nil
	}
	// 教員以外には教員用レポートを返さない
	if report != nil && role != "teacher" {
		sanitized := *report
		sanitized.TeacherReportJSON = ""
		report = &sanitized
	}
	return &InterviewDetailResponse{
		Session:    *toSessionResponse(session),
		Utterances: utterances,
		Report:     report,
	}, nil
}

func (s *InterviewService) GetReport(userID uint, sessionID uint) (*models.InterviewReport, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	report, err := s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return report, nil
}

// PhraseSuggestion は面接回答中の弱い表現と言い換え候補を表す。
type PhraseSuggestion struct {
	Original    string   `json:"original"`
	Suggestions []string `json:"suggestions"`
}

// GetPhraseSuggestions はセッションのユーザー発話を分析し、言い換え提案を返す。
func (s *InterviewService) GetPhraseSuggestions(ctx context.Context, userID uint, sessionID uint) ([]PhraseSuggestion, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}
	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return nil, err
	}
	var userTexts []string
	for _, u := range utterances {
		if u.Role == "user" {
			t := strings.TrimSpace(u.Text)
			if t != "" {
				userTexts = append(userTexts, t)
			}
		}
	}
	if len(userTexts) == 0 {
		return []PhraseSuggestion{}, nil
	}
	transcript := strings.Join(userTexts, "\n")
	systemPrompt := "あなたは就活面接コーチです。応募者の発言から改善すべき曖昧・抽象的・弱い表現を抽出し、より具体的・主体的・印象的な言い換えを提案してください。"
	userPrompt := fmt.Sprintf(`以下は就活面接における応募者の発言です。
改善が有効な表現を最大5件抽出し、それぞれに2〜3件の言い換え候補を提示してください。

出力はJSONのみ（マークダウン不可）で以下の形式を厳守してください:
{"suggestions": [{"original": "元の表現", "suggestions": ["言い換え1", "言い換え2"]}, ...]}

応募者発言:
%s`, transcript)

	model := getEnv("INTERVIEW_REPORT_MODEL", "")
	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.5, 1000, model)
	if err != nil {
		return nil, err
	}
	raw = ExtractJSONObject(raw)
	var payload struct {
		Suggestions []PhraseSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse suggestions: %w", err)
	}
	return payload.Suggestions, nil
}

// InterviewTrendPoint は面接トレンド分析の1データポイント。
type InterviewTrendPoint struct {
	SessionID     uint      `json:"session_id"`
	CreatedAt     time.Time `json:"created_at"`
	Logic         *float64  `json:"logic"`
	Specificity   *float64  `json:"specificity"`
	Ownership     *float64  `json:"ownership"`
	Communication *float64  `json:"communication"`
	Enthusiasm    *float64  `json:"enthusiasm"`
}

// GetTrend は指定ユーザーの完了済み面接セッションのスコア時系列を返す。
// sessions は古い順（昇順）で返却されるため、フロントエンドでそのままグラフに使える。
func (s *InterviewService) GetTrend(userID uint, limit int) ([]InterviewTrendPoint, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	// 新しい順で取得し、後で逆順にする（古い順でグラフ描画するため）
	sessions, err := s.sessionRepo.ListFinishedByUser(userID, limit)
	if err != nil {
		return nil, err
	}
	// 古い順に並べ替え
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	points := make([]InterviewTrendPoint, 0, len(sessions))
	for _, session := range sessions {
		report, err := s.reportRepo.FindBySessionID(session.ID)
		if err != nil {
			// レポート未生成のセッションはスキップ
			continue
		}
		var scores map[string]float64
		if err := json.Unmarshal([]byte(report.ScoresJSON), &scores); err != nil {
			continue
		}
		pt := InterviewTrendPoint{
			SessionID: session.ID,
			CreatedAt: session.CreatedAt,
		}
		if v, ok := scores["logic"]; ok {
			vv := v
			pt.Logic = &vv
		}
		if v, ok := scores["specificity"]; ok {
			vv := v
			pt.Specificity = &vv
		}
		if v, ok := scores["ownership"]; ok {
			vv := v
			pt.Ownership = &vv
		}
		if v, ok := scores["communication"]; ok {
			vv := v
			pt.Communication = &vv
		}
		if v, ok := scores["enthusiasm"]; ok {
			vv := v
			pt.Enthusiasm = &vv
		}
		points = append(points, pt)
	}
	return points, nil
}

func (s *InterviewService) CreateRealtimeToken(ctx context.Context, userID uint, sessionID uint) (string, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return "", err
	}
	if !s.isAllowed(userID, session.UserID) {
		return "", errors.New("forbidden")
	}
	if session.Status == "finished" {
		return "", errors.New("session already finished")
	}
	if s.realtimeUsageService != nil {
		allowed, active, maxAllowed, err := s.realtimeUsageService.CanOpenNewConnection()
		if err != nil {
			return "", err
		}
		if !allowed {
			return "", fmt.Errorf("realtime capacity exceeded: active=%d limit=%d", active, maxAllowed)
		}
	}
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}
	gender := session.InterviewerGender
	if gender == "" {
		gender = "female"
	}
	model := getEnv("OPENAI_REALTIME_MODEL", "gpt-4o-realtime-preview")
	voice := realtimeVoiceForLangAndGender(lang, gender)
	transcribeModel := getEnv("OPENAI_REALTIME_TRANSCRIBE_MODEL", "gpt-4o-mini-transcribe")
	maxTokens := getIntEnv("OPENAI_REALTIME_MAX_OUTPUT_TOKENS", 120)
	req := openai.RealtimeSessionRequest{
		Model:        model,
		Modalities:   []string{"audio"},
		Voice:        voice,
		Instructions: buildRealtimeInstructions(lang),
		InputAudioTranscription: map[string]interface{}{
			"model": transcribeModel,
		},
		TurnDetection: map[string]interface{}{
			"type":                "server_vad",
			"threshold":           0.5,
			"silence_duration_ms": 700,
			"prefix_padding_ms":   300,
			"create_response":     true,
		},
		MaxResponseOutputTokens: maxTokens,
	}
	resp, err := s.openaiClient.CreateRealtimeClientSecret(ctx, req)
	if err != nil {
		return "", err
	}
	if s.realtimeUsageService != nil {
		if err := s.realtimeUsageService.EnsureSessionStarted(userID, sessionID); err != nil {
			return "", err
		}
	}
	return resp.ClientSecret.Value, nil
}

func (s *InterviewService) estimateCost(start, end *time.Time) float64 {
	if start == nil || end == nil {
		return 0
	}
	minutes := end.Sub(*start).Minutes()
	if minutes < 0 {
		return 0
	}
	rate := getFloatEnv("INTERVIEW_COST_PER_MIN_USD", 0.18)
	return minutes * rate
}

// TurnResult は1ターンの結果（AIテキスト + TTS音声バイト列）
type TurnResult struct {
	UserText string
	AIText   string
	Audio    []byte
}

// Turn はユーザー音声を受け取り、STT→Chat→TTSを実行してTurnResultを返します
func (s *InterviewService) Turn(ctx context.Context, userID uint, sessionID uint, audioData []byte, history []map[string]string, companyName, companyReading, position, companyInfo, companyType string) (*TurnResult, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}

	// STT: Whisper でユーザー音声をテキスト化
	userText, err := s.openaiClient.Transcribe(ctx, audioData, "audio.webm")
	if err != nil {
		return nil, fmt.Errorf("transcribe error: %w", err)
	}
	if strings.TrimSpace(userText) == "" {
		userText = "（聞き取れませんでした）"
	}

	// 読み仮名が未指定の場合はWeb検索で自動取得
	if companyName != "" && companyReading == "" {
		companyReading = s.lookupCompanyReading(ctx, companyName)
	}

	// 自社開発企業の場合は公式サイト情報を取得
	if companyType == "general" && companyName != "" && companyInfo == "" {
		companyInfo = s.lookupCompanyProfile(ctx, companyName)
	}

	// 履歴にユーザー発言を追加
	history = append(history, map[string]string{"role": "user", "content": userText})

	// Chat: 面接官として返答生成
	systemPrompt := buildInterviewSystemPrompt(companyName, companyReading, position, companyInfo, companyType)
	if s.crossFeature != nil {
		if profileCtx := s.crossFeature.BuildInterviewContextFromUser(userID); profileCtx != "" {
			systemPrompt = profileCtx + "\n" + systemPrompt
		}
	}
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPrompt, history)
	if err != nil {
		return nil, fmt.Errorf("chat error: %w", err)
	}

	// TTS: AI返答を音声化
	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, aiText, voice)
	if err != nil {
		return nil, fmt.Errorf("tts error: %w", err)
	}

	return &TurnResult{UserText: userText, AIText: aiText, Audio: audio}, nil
}

// StartTurn は面接開始の最初のAI発話を生成します
func (s *InterviewService) StartTurn(ctx context.Context, userID uint, sessionID uint, companyName, companyReading, position, companyInfo, companyType string) (*TurnResult, error) {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	if !s.isAllowed(userID, session.UserID) {
		return nil, errors.New("forbidden")
	}

	// 読み仮名が未指定の場合はWeb検索で自動取得
	if companyName != "" && companyReading == "" {
		companyReading = s.lookupCompanyReading(ctx, companyName)
	}

	// 自社開発企業の場合は公式サイト情報を取得
	if companyType == "general" && companyName != "" && companyInfo == "" {
		companyInfo = s.lookupCompanyProfile(ctx, companyName)
	}

	systemPromptStart := buildInterviewSystemPrompt(companyName, companyReading, position, companyInfo, companyType)
	if s.crossFeature != nil {
		if profileCtx := s.crossFeature.BuildInterviewContextFromUser(userID); profileCtx != "" {
			systemPromptStart = profileCtx + "\n" + systemPromptStart
		}
	}
	aiText, err := s.openaiClient.ChatInterview(ctx, systemPromptStart, []map[string]string{
		{"role": "user", "content": "面接を開始してください。最初の自己紹介・志望動機の質問からお願いします。"},
	})
	if err != nil {
		return nil, fmt.Errorf("chat error: %w", err)
	}

	voice := ttsVoiceForGenderAndLang(session.InterviewerGender, session.Language)
	audio, err := s.openaiClient.TTS(ctx, aiText, voice)
	if err != nil {
		return nil, fmt.Errorf("tts error: %w", err)
	}

	return &TurnResult{AIText: aiText, Audio: audio}, nil
}

// lookupCompanyReading はWeb検索を使って企業名の日本語読み（ふりがな）を取得します。
// 取得に失敗した場合は空文字を返します（エラーは無視）。
func (s *InterviewService) lookupCompanyReading(ctx context.Context, companyName string) string {
	query := fmt.Sprintf("「%s」の正しい日本語読み（ふりがな）をカタカナで1行だけ答えてください。", companyName)
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := s.openaiClient.WebSearchQuery(ctxTimeout, query)
	if err != nil {
		return ""
	}
	// 最初の行だけ抽出し、余分な記号や空白を除去
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) == 0 {
		return ""
	}
	reading := strings.TrimSpace(lines[0])
	// 句読点・括弧・引用符等を除去
	reading = strings.Trim(reading, "「」『』（）()。、・ ")
	return reading
}

// lookupCompanyProfile はWeb検索を使って企業の公式サイト情報（理念・求める人物像・事業内容）を取得します。
// 取得に失敗した場合は空文字を返します（エラーは無視）。
func (s *InterviewService) lookupCompanyProfile(ctx context.Context, companyName string) string {
	query := fmt.Sprintf("%s 公式サイト 求める人物像 企業理念 事業内容", companyName)
	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := s.openaiClient.WebSearchQuery(ctxTimeout, query)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(result)
}

func buildInterviewSystemPrompt(companyName, companyReading, position, companyInfo, companyType string) string {
	base := `あなたは日本語の就活面接官です。以下を守ってください。
- 1回の返答は2〜3文以内で短くまとめる
- 必ず1つの質問で締めくくる
- 応募者が話しやすいよう具体的に深掘りする
- 評価・講評は面接終了まで行わない
- 企業名を発話する際は、英字・略語はカタカナの正しい読み方で読んでください`

	if companyName != "" || position != "" {
		base += "\n\n【面接情報】"
		if companyName != "" {
			companyLabel := companyName
			if companyReading != "" {
				companyLabel += "（読み: " + companyReading + "）"
			}
			base += "\n志望企業: " + companyLabel
		}
		if position != "" {
			base += "\n応募職種: " + position
		}
		if companyInfo != "" {
			if companyType == "general" {
				base += "\n\n【企業研究情報（公式サイトより）】\n" + companyInfo
			} else {
				base += "\n企業概要: " + companyInfo
			}
		}
		base += "\n\n上記の企業・職種に合わせた質問を行ってください。"
	}

	if companyType == "sier" {
		base += `

【SIer企業向け質問ガイドライン】
- 常駐先での顧客折衝・要件ヒアリング経験を深掘りする
- 上流工程（要件定義・基本設計）への関与実績を確認する
- ウォーターフォール・アジャイルどちらの経験があるか確認する
- IPA資格や技術資格の取得状況・今後の学習意欲を聞く
- 多様な現場・技術スタックへの適応力を問う`
	}

	return strings.TrimSpace(base)
}

func (s *InterviewService) isAllowed(actorID uint, ownerID uint) bool {
	if actorID == ownerID {
		return true
	}
	user, err := s.userRepo.GetUserByID(actorID)
	if err != nil || user == nil {
		return false
	}
	return user.IsAdmin
}

// buildTranscript formats utterances into a plain-text transcript for the LLM prompt.
// AI turns are labeled "Interviewer" and user turns are labeled "User".
func BuildTranscript(utterances []models.InterviewUtterance) string {
	var b strings.Builder
	for _, u := range utterances {
		role := "User"
		if u.Role == "ai" {
			role = "Interviewer"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(u.Text))
		b.WriteString("\n")
	}
	return b.String()
}

// extractJSONObject strips surrounding markdown code fences and extracts the
// outermost JSON object from an LLM response.
// Some models wrap their output in ```json ... ``` even when instructed not to.
func ExtractJSONObject(raw string) string {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return s
}

func (s *InterviewService) generateReport(ctx context.Context, sessionID uint) error {
	session, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}

	utterances, err := s.utterRepo.FindBySessionID(sessionID)
	if err != nil {
		return err
	}
	if len(utterances) == 0 {
		// utterances が0件の場合は空レポートを保存して正常終了
		empty := &models.InterviewReport{
			SessionID:         sessionID,
			SummaryText:       "発話データがありませんでした。",
			ScoresJSON:        `{"logic":0,"specificity":0,"ownership":0,"communication":0,"enthusiasm":0}`,
			EvidenceJSON:      `{}`,
			StrengthsJSON:     `[]`,
			ImprovementsJSON:  `[]`,
			TeacherReportJSON: `{}`,
		}
		return s.reportRepo.Upsert(empty)
	}
	transcript := BuildTranscript(utterances)
	systemPrompt := buildReportSystemPrompt(lang)
	userPrompt := fmt.Sprintf(`以下の面接ログを読み、下記の評価基準に従ってJSONのみで出力してください。
出力言語: %s

## 評価基準（各スコアは0〜5の整数）
- logic（論理性）: 回答が筋道立っているか、主張に一貫性があるか
- specificity（具体性）: 具体的なエピソードや数値が含まれているか
- ownership（主体性）: 「私が〜した」という自分起点の表現があるか
- communication（コミュニケーション力）: 簡潔・明確に伝えられているか、聞き返しが少ないか
- enthusiasm（積極性・熱意）: 志望動機や意欲が伝わっているか

## 出力フォーマット（このキーと型を厳守してください）
{
  "summary": "面接全体の総合評価コメント（2〜3文、生徒向けのやさしい言葉で）",
  "scores": {"logic": 3, "specificity": 2, "ownership": 4, "communication": 3, "enthusiasm": 4},
  "evidence": {
    "logic": "論理性の根拠となった発言",
    "specificity": "具体性の根拠となった発言",
    "ownership": "主体性の根拠となった発言",
    "communication": "コミュニケーション力の根拠となった発言",
    "enthusiasm": "積極性・熱意の根拠となった発言"
  },
  "strengths": ["強み1", "強み2", "強み3"],
  "improvements": ["改善点1", "改善点2", "改善点3"],
  "teacher": {
    "overall_comment": "教員向け総評（指導観点・クラス内での位置づけ等）",
    "detailed_evidence": {"logic": "詳細な根拠と指導ポイント", "specificity": "詳細な根拠と指導ポイント", "ownership": "詳細な根拠と指導ポイント"},
    "coaching_points": ["具体的な改善指導ポイント1", "ポイント2", "ポイント3"],
    "strengths_for_teacher": ["指導者が把握すべき強み1", "強み2"],
    "next_steps": ["次回面接に向けた具体的な課題1", "課題2"]
  }
}

※ scoresは実際の会話内容に基づいて正直に採点してください（全て同じ値は避ける）。
※ strengths/improvementsは各2〜4件のリスト形式で具体的に記述してください。
※ teacher以下は教員専用の詳細情報として出力してください。

Interview transcript:
%s`, lang, transcript)

	model := getEnv("INTERVIEW_REPORT_MODEL", "")
	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.4, 2000, model)
	if err != nil {
		return err
	}
	type teacherReport struct {
		OverallComment      string            `json:"overall_comment"`
		DetailedEvidence    map[string]string `json:"detailed_evidence"`
		CoachingPoints      []string          `json:"coaching_points"`
		StrengthsForTeacher []string          `json:"strengths_for_teacher"`
		NextSteps           []string          `json:"next_steps"`
	}
	type reportPayload struct {
		Summary      string            `json:"summary"`
		Scores       map[string]int    `json:"scores"`
		Evidence     map[string]string `json:"evidence"`
		Strengths    []string          `json:"strengths"`
		Improvements []string          `json:"improvements"`
		Teacher      *teacherReport    `json:"teacher"`
	}
	var payload reportPayload
	cleaned := ExtractJSONObject(raw)
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return fmt.Errorf("invalid report json: %w", err)
	}
	scoresJSON, _ := json.Marshal(payload.Scores)
	evidenceJSON, _ := json.Marshal(payload.Evidence)
	strengthsJSON, _ := json.Marshal(payload.Strengths)
	improvementsJSON, _ := json.Marshal(payload.Improvements)
	teacherJSON := []byte("{}")
	if payload.Teacher != nil {
		teacherJSON, _ = json.Marshal(payload.Teacher)
	}

	report := &models.InterviewReport{
		SessionID:         sessionID,
		SummaryText:       payload.Summary,
		ScoresJSON:        string(scoresJSON),
		EvidenceJSON:      string(evidenceJSON),
		StrengthsJSON:     string(strengthsJSON),
		ImprovementsJSON:  string(improvementsJSON),
		TeacherReportJSON: string(teacherJSON),
	}
	if err := s.reportRepo.Upsert(report); err != nil {
		return err
	}

	// 面接スコアを UserWeightScore に反映（crossFeature が設定済みの場合のみ）
	if s.crossFeature != nil {
		chatSessionID := fmt.Sprintf("interview-%d", session.UserID)
		if err := s.crossFeature.UpdateScoresFromInterviewReport(session.UserID, chatSessionID, report); err != nil {
			// スコア反映失敗はレポート生成を失敗扱いにしない
			fmt.Printf("[CrossFeature] interview score update failed for session %d: %v\n", sessionID, err)
		}
	}
	return nil
}

// SendReportEmail 面接レポートをメールで送信
func (s *InterviewService) SendReportEmail(userID, sessionID uint) error {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if user.IsGuest {
		return errors.New("guest users cannot receive email reports")
	}

	report, err := s.reportRepo.FindBySessionID(sessionID)
	if err != nil {
		return errors.New("report not found")
	}

	var scores map[string]int
	json.Unmarshal([]byte(report.ScoresJSON), &scores)
	var evidence map[string]string
	json.Unmarshal([]byte(report.EvidenceJSON), &evidence)

	summary := strings.Split(strings.TrimSpace(report.SummaryText), "\n")
	var filtered []string
	for _, line := range summary {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, strings.TrimSpace(line))
		}
	}

	data := InterviewReportEmailData{
		SessionID:  fmt.Sprintf("%d", sessionID),
		Summary:    filtered,
		LogicScore: scores["logic"],
		SpecScore:  scores["specificity"],
		OwnScore:   scores["ownership"],
		LogicEvid:  evidence["logic"],
		SpecEvid:   evidence["specificity"],
		OwnEvid:    evidence["ownership"],
	}
	return s.emailService.SendInterviewReport(user, data)
}

func toSessionResponse(session *models.InterviewSession) *InterviewSessionResponse {
	lang := session.Language
	if lang == "" {
		lang = "ja"
	}
	gender := session.InterviewerGender
	if gender == "" {
		gender = "female"
	}
	return &InterviewSessionResponse{
		ID:                session.ID,
		UserID:            session.UserID,
		Status:            session.Status,
		Language:          lang,
		InterviewerGender: gender,
		StartedAt:         session.StartedAt,
		EndedAt:           session.EndedAt,
		EstimatedCostUSD:  session.EstimatedCostUSD,
		TemplateVersion:   session.TemplateVersion,
		CreatedAt:         session.CreatedAt,
		UpdatedAt:         session.UpdatedAt,
	}
}

func toSessionResponses(sessions []models.InterviewSession) []InterviewSessionResponse {
	out := make([]InterviewSessionResponse, 0, len(sessions))
	for i := range sessions {
		out = append(out, *toSessionResponse(&sessions[i]))
	}
	return out
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func getIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getFloatEnv(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

// buildReportSystemPrompt 言語コードに応じたレポート生成用システムプロンプトを返す。
func buildReportSystemPrompt(lang string) string {
	known := map[string]string{
		"ja": "あなたは就活面接のアシスタントです。面接ログを読み、要約・評価をJSONで返してください。",
		"en": "You are a job interview assessment assistant. Read the interview transcript and return evaluation as JSON.",
		"zh": "你是一位求职面试评估助手。请阅读面试记录并以JSON格式返回评估结果。",
		"ko": "당신은 취업 면접 평가 어시스턴트입니다. 면접 기록을 읽고 JSON 형식으로 평가를 반환하세요。",
		"fr": "Vous êtes un assistant d'évaluation d'entretien d'embauche. Lisez la transcription et retournez l'évaluation en JSON.",
		"es": "Eres un asistente de evaluación de entrevistas de trabajo. Lee la transcripción y devuelve la evaluación en JSON.",
		"de": "Sie sind ein Assistent zur Bewertung von Vorstellungsgesprächen. Lesen Sie das Transkript und geben Sie die Bewertung als JSON zurück.",
		"pt": "Você é um assistente de avaliação de entrevistas de emprego. Leia a transcrição e retorne a avaliação em JSON.",
		"it": "Sei un assistente per la valutazione dei colloqui di lavoro. Leggi la trascrizione e restituisci la valutazione in JSON.",
		"ar": "أنت مساعد تقييم مقابلات العمل. اقرأ النص وأعد التقييم بصيغة JSON.",
		"ru": "Вы ассистент по оценке собеседований. Прочитайте транскрипт и верните оценку в формате JSON.",
		"hi": "आप नौकरी साक्षात्कार मूल्यांकन सहायक हैं। साक्षात्कार का विवरण पढ़ें और मूल्यांकन JSON में लौटाएं।",
		"th": "คุณเป็นผู้ช่วยประเมินการสัมภาษณ์งาน อ่านบทสนทนาแล้วส่งคืนการประเมินในรูปแบบ JSON",
		"vi": "Bạn là trợ lý đánh giá phỏng vấn tuyển dụng. Đọc bản ghi và trả về đánh giá dưới dạng JSON.",
		"id": "Anda adalah asisten evaluasi wawancara kerja. Baca transkrip dan kembalikan evaluasi dalam format JSON.",
		"tr": "Siz bir iş görüşmesi değerlendirme asistanısınız. Metni okuyun ve değerlendirmeyi JSON formatında döndürün.",
	}
	if prompt, ok := known[lang]; ok {
		return prompt
	}
	return fmt.Sprintf("You are a job interview assessment assistant. Read the interview transcript and return evaluation as JSON. Use language code \"%s\" for the summary and evidence fields.", lang)
}

// ttsVoiceForGenderAndLang 性別と言語に応じたTTSボイスを返す。
// 参照: https://note.com/affiwriting/n/nc2a665c234a7
// 男性: onyx (深みがあり権威のある声)
// 女性: shimmer (クリアで表現力豊かな女性の声)
func ttsVoiceForGenderAndLang(gender, lang string) string {
	switch gender {
	case "male":
		return "onyx"
	default: // female
		return "shimmer"
	}
}

// realtimeVoiceForLangAndGender 言語・性別コードに応じた推奨ボイスを返す。
// 環境変数 OPENAI_REALTIME_VOICE が設定されている場合はそちらを優先する。
func realtimeVoiceForLangAndGender(lang, gender string) string {
	if v := getEnv("OPENAI_REALTIME_VOICE", ""); v != "" {
		return v
	}
	return ttsVoiceForGenderAndLang(gender, lang)
}

// buildRealtimeInstructions 面接官AIへのシステムプロンプトを返す。
// デフォルトは日本語で進行し、面接者から別言語を求められた場合は即座に切り替える。
func buildRealtimeInstructions(_ string) string {
	return strings.TrimSpace(`あなたはプロの就活面接官です。以下のルールに従ってください。

【言語対応】
- デフォルトは日本語で面接を行う
- 面接者から別の言語での面接を求められた場合（例：「英語でお願いします」「Please switch to English」「请用中文」など）は、即座にその言語に切り替えて面接を継続する
- 一度切り替えた言語は、面接者から変更を求められるまで維持する

【質問の意図が伝わらない場合】
- 面接者から「意味がわかりません」「質問の意図を教えてください」「もう少し詳しく教えてください」などの発言があった場合は、同じ質問を別の言い方で言い換えるか、具体的な例を添えて再度問いかける
- 言い換えても理解が難しそうな場合は「では少し視点を変えて〜」と切り出し、関連する別の質問に移る
- 面接者が質問に詰まっている場合は「焦らずに考えてみてください」と一言添えてから待つ

【面接の進め方】
- 1回の発話は短く、質問中心にする
- 詳細な講評・フィードバックは面接終了後に行う
- 面接者が話しやすいよう、具体的に深掘りする`)
}
