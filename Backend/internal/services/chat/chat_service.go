package chat

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/growth"
	"context"
	"fmt"
	"log"
)

type ChatService struct {
	aiClient                *openai.Client
	questionWeightRepo      repository.QuestionWeightRepository
	chatMessageRepo         repository.ChatMessageRepository
	userWeightScoreRepo     repository.UserWeightScoreRepository
	aiGeneratedQuestionRepo repository.AIGeneratedQuestionRepository
	predefinedQuestionRepo  repository.PredefinedQuestionRepository
	jobCategoryRepo         repository.JobCategoryRepository
	userRepo                repository.UserRepository
	userEmbeddingRepo       repository.UserEmbeddingRepository
	jobEmbeddingRepo        repository.JobCategoryEmbeddingRepository
	phaseRepo               repository.AnalysisPhaseRepository
	progressRepo            repository.UserAnalysisProgressRepository
	sessionValidationRepo   repository.SessionValidationRepository
	conversationContextRepo repository.ConversationContextRepository
	answerEvaluator         *AnswerEvaluator
	jobValidator            *JobCategoryValidator
	growthRecorder          growth.Recorder
	questionEmb             questionEmbCache
}

// SetGrowthRecorder は、スコアが動いたあとに履歴を1行残す。未設定なら何もしない。
func (s *ChatService) SetGrowthRecorder(r growth.Recorder) {
	if s == nil {
		return
	}
	s.growthRecorder = r
}

func (s *ChatService) recordGrowth(userID uint, source, sessionID string) {
	if s == nil || s.growthRecorder == nil {
		return
	}
	s.growthRecorder.Record(userID, source, sessionID)
}

func NewChatService(
	aiClient *openai.Client,
	questionWeightRepo repository.QuestionWeightRepository,
	chatMessageRepo repository.ChatMessageRepository,
	userWeightScoreRepo repository.UserWeightScoreRepository,
	aiGeneratedQuestionRepo repository.AIGeneratedQuestionRepository,
	predefinedQuestionRepo repository.PredefinedQuestionRepository,
	jobCategoryRepo repository.JobCategoryRepository,
	userRepo repository.UserRepository,
	userEmbeddingRepo repository.UserEmbeddingRepository,
	jobEmbeddingRepo repository.JobCategoryEmbeddingRepository,
	phaseRepo repository.AnalysisPhaseRepository,
	progressRepo repository.UserAnalysisProgressRepository,
	sessionValidationRepo repository.SessionValidationRepository,
	conversationContextRepo repository.ConversationContextRepository,
) *ChatService {
	return &ChatService{
		aiClient:                aiClient,
		questionWeightRepo:      questionWeightRepo,
		chatMessageRepo:         chatMessageRepo,
		userWeightScoreRepo:     userWeightScoreRepo,
		aiGeneratedQuestionRepo: aiGeneratedQuestionRepo,
		predefinedQuestionRepo:  predefinedQuestionRepo,
		jobCategoryRepo:         jobCategoryRepo,
		userRepo:                userRepo,
		userEmbeddingRepo:       userEmbeddingRepo,
		jobEmbeddingRepo:        jobEmbeddingRepo,
		phaseRepo:               phaseRepo,
		progressRepo:            progressRepo,
		sessionValidationRepo:   sessionValidationRepo,
		conversationContextRepo: conversationContextRepo,
		answerEvaluator:         NewAnswerEvaluatorWithLLM(aiClient),
		jobValidator:            NewJobCategoryValidator(aiClient, jobCategoryRepo),
	}
}

// RecalculationModel はチャットスコア再計算の概算に使うモデルと課金有無を返す。
func (s *ChatService) RecalculationModel() (string, bool) {
	if s.aiClient == nil {
		return "", false
	}
	provider, _, _ := s.aiClient.Providers()
	if provider == "openai" {
		return s.aiClient.DefaultModel, true
	}
	// ローカル推論先でも、障害時は OpenAI へ再送されて課金される。
	// 管理者が承認する金額を超えないよう、全件が再送された場合で見積もる。
	if model, ok := s.aiClient.TextFallbackModel(); ok {
		return model, true
	}
	return s.aiClient.DefaultModel, false
}

// ChatRequest チャットリクエスト
type ChatRequest struct {
	UserID        uint   `json:"user_id"`
	SessionID     string `json:"session_id"`
	Message       string `json:"message"`
	IndustryID    uint   `json:"industry_id"`
	JobCategoryID uint   `json:"job_category_id"`
	// ConsultIntent はホームの「キャリア相談」で選んだ意図（任意。送らない既存のクライアントは従来どおり動く）。
	// セッションの最初の回答でだけ使う（ConsultIntent* を参照）。
	ConsultIntent string `json:"consult_intent,omitempty"`
}

// ChatResponse チャットレスポンス
type ChatResponse struct {
	Response            string                           `json:"response"`
	QuestionWeightID    uint                             `json:"question_weight_id,omitempty"`
	CurrentScores       []models.UserWeightScoreResponse `json:"current_scores,omitempty"`
	CurrentPhase        *PhaseProgress                   `json:"current_phase,omitempty"`
	AllPhases           []PhaseProgress                  `json:"all_phases,omitempty"`
	IsComplete          bool                             `json:"is_complete"`
	IsTerminated        bool                             `json:"is_terminated,omitempty"`
	SuggestRestart      bool                             `json:"suggest_restart,omitempty"`
	InvalidAnswerCount  int                              `json:"invalid_answer_count,omitempty"`
	TotalQuestions      int                              `json:"total_questions"`
	AnsweredQuestions   int                              `json:"answered_questions"`
	EvaluatedCategories int                              `json:"evaluated_categories"`
	TotalCategories     int                              `json:"total_categories"`
	Summary             *SessionSummary                  `json:"summary,omitempty"`
	JobCategoryID       uint                             `json:"job_category_id,omitempty"`
	// JudgmentPercent は「判定の進み具合」（0〜100）。質問数は固定しないので、画面の進捗はこれを使う。
	// 判定できたとき（完了）だけ 100 になる。
	JudgmentPercent int `json:"judgment_percent"`
	// SuggestProvisional は、目安の回答数を超えても判定できていないときに true。
	// 画面は「いまの暫定の結果を見る」を出す（質問は続けられる）。
	SuggestProvisional bool `json:"suggest_provisional,omitempty"`
}

// PhaseProgress フェーズ進捗情報
type PhaseProgress struct {
	PhaseID         uint    `json:"phase_id"`
	PhaseName       string  `json:"phase_name"`
	DisplayName     string  `json:"display_name"`
	QuestionsAsked  int     `json:"questions_asked"`
	ValidAnswers    int     `json:"valid_answers"`
	CompletionScore float64 `json:"completion_score"`
	IsCompleted     bool    `json:"is_completed"`
	MinQuestions    int     `json:"min_questions"`
	MaxQuestions    int     `json:"max_questions"`
}

// SessionSummary セッション完了時に返す要約（強み・注意点・おすすめの働き方）
type SessionSummary struct {
	Strengths               string `json:"strengths"`
	Weaknesses              string `json:"weaknesses"`
	RecommendedWorkingStyle string `json:"recommended_working_style"`
}

// ProcessChat チャット処理のメインロジック
func (s *ChatService) ProcessChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// 学生の回答を外部 AI へ送る前に、個人を特定できる語を伏字にする（下流の全呼び出しに効く）
	ctx = s.withPIIGuard(ctx, req.UserID)

	// セッション開始の特殊処理
	if req.Message == "START_SESSION" {
		return s.handleSessionStart(ctx, req)
	}

	// セッション終了チェック
	isTerminated, err := s.sessionValidationRepo.IsTerminated(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check session status: %w", err)
	}
	if isTerminated {
		terminationMsg := "このセッションは終了しています。不適切な回答が3回続いたため、チャットを終了しました。新しいセッションを開始してください。"
		assistantMsg := &models.ChatMessage{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Role:      "assistant",
			Content:   terminationMsg,
		}
		if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
			log.Printf("Warning: failed to save termination message: %v\n", err)
		}
		return &ChatResponse{
			Response:     terminationMsg,
			IsComplete:   true,
			IsTerminated: true,
		}, nil
	}

	// 1. ユーザーのメッセージを保存
	userMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := s.chatMessageRepo.Create(userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// 2. 会話履歴を取得（ユーザーメッセージ保存後に取得）
	//    user_id でスコープする。他人のセッションIDを渡されても他人の発言が
	//    LLM のコンテキストへ入らないようにする（#1156 多層防御）
	history, err := s.chatMessageRepo.FindRecentBySessionIDForUser(req.SessionID, req.UserID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat history: %w", err)
	}

	// 2-1. 職種の解決（未設定なら判定し、セッションに保存）
	jobResolution, err := s.resolveJobCategoryForChat(ctx, req, history)
	if err != nil {
		return nil, err
	}
	if jobResolution.earlyResponse != nil {
		return jobResolution.earlyResponse, nil
	}

	// 2.5. 回答の妥当性チェック（保存後のhistoryを使用）
	handled, response, err := s.checkAnswerValidity(ctx, history, req.Message, req.UserID, req.SessionID)
	if err != nil {
		return nil, err
	}
	if handled {
		return s.handleInvalidAnswerFlow(ctx, req, history, response)
	}

	// 有効な回答の場合のみ、以降の処理を実行
	// 2.6. 現在のフェーズを取得または開始
	currentPhase, err := s.getCurrentOrNextPhase(ctx, req.UserID, req.SessionID)
	extraRound := false
	if err != nil {
		if err.Error() != "all phases completed" {
			return nil, fmt.Errorf("failed to get current phase: %w", err)
		}
		// 全フェーズを終えても判定できていなければ、最後のフェーズの枠で質問を続ける（延長ラウンド）。
		// 判定できていれば、従来どおり完了にする。
		if s.judgmentFor(req.UserID, req.SessionID).Ready {
			return s.handleAllPhasesCompletedFromPhaseError(ctx, req, history)
		}
		last, lastErr := s.getLastPhaseProgress(req.UserID, req.SessionID)
		if lastErr != nil {
			return nil, fmt.Errorf("failed to get last phase for extra round: %w", lastErr)
		}
		currentPhase, extraRound = last, true
	}

	allPhases, err := s.phaseRepo.FindAll()
	if err != nil {
		log.Printf("Warning: failed to get phases: %v\n", err)
	}
	completedProgresses, _ := s.progressRepo.FindByUserAndSession(req.UserID, req.SessionID)
	phaseByID := make(map[uint]*models.AnalysisPhase, len(allPhases))
	for i := range allPhases {
		phaseByID[allPhases[i].ID] = &allPhases[i]
	}
	if resp, handled := s.handleAllPhasesCompletedFromCount(ctx, req, history, allPhases); handled {
		return resp, nil
	}

	return s.processAnswerAndNextQuestion(ctx, processQuestionInput{
		req:                 req,
		history:             history,
		jobCategoryID:       jobResolution.jobCategoryID,
		jobJustResolved:     jobResolution.jobJustResolved,
		currentPhase:        currentPhase,
		allPhases:           allPhases,
		completedProgresses: completedProgresses,
		phaseByID:           phaseByID,
		extraRound:          extraRound,
	})
}
