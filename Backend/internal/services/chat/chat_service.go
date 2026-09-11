package chat

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
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

// ChatRequest チャットリクエスト
type ChatRequest struct {
	UserID        uint   `json:"user_id"`
	SessionID     string `json:"session_id"`
	Message       string `json:"message"`
	IndustryID    uint   `json:"industry_id"`
	JobCategoryID uint   `json:"job_category_id"`
}

// ChatResponse チャットレスポンス
type ChatResponse struct {
	Response            string                   `json:"response"`
	QuestionWeightID    uint                     `json:"question_weight_id,omitempty"`
	CurrentScores       []entity.UserWeightScore `json:"current_scores,omitempty"`
	CurrentPhase        *PhaseProgress           `json:"current_phase,omitempty"`
	AllPhases           []PhaseProgress          `json:"all_phases,omitempty"`
	IsComplete          bool                     `json:"is_complete"`
	IsTerminated        bool                     `json:"is_terminated,omitempty"`
	SuggestRestart      bool                     `json:"suggest_restart,omitempty"`
	InvalidAnswerCount  int                      `json:"invalid_answer_count,omitempty"`
	TotalQuestions      int                      `json:"total_questions"`
	AnsweredQuestions   int                      `json:"answered_questions"`
	EvaluatedCategories int                      `json:"evaluated_categories"`
	TotalCategories     int                      `json:"total_categories"`
	Summary             *SessionSummary          `json:"summary,omitempty"`
	JobCategoryID       uint                     `json:"job_category_id,omitempty"`
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
	history, err := s.chatMessageRepo.FindRecentBySessionID(req.SessionID, 100)
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
	if err != nil {
		if err.Error() == "all phases completed" {
			return s.handleAllPhasesCompletedFromPhaseError(req, history)
		}
		return nil, fmt.Errorf("failed to get current phase: %w", err)
	}

	allPhases, err := s.phaseRepo.FindAll()
	if err != nil {
		log.Printf("Warning: failed to get phases: %v\n", err)
	}
	completedProgresses, _ := s.progressRepo.FindByUserAndSession(req.UserID, req.SessionID)
	phaseByID := make(map[uint]*entity.AnalysisPhase, len(allPhases))
	for i := range allPhases {
		phaseByID[allPhases[i].ID] = &allPhases[i]
	}
	if resp, handled := s.handleAllPhasesCompletedFromCount(req, history, allPhases, completedProgresses, phaseByID); handled {
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
	})
}
