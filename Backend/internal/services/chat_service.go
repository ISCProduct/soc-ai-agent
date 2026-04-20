package services

import (
	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/openai"
	"context"
	"fmt"
	"log"
	"strings"
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
		answerEvaluator:         NewAnswerEvaluator(),
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
	InvalidAnswerCount  int                      `json:"invalid_answer_count,omitempty"`
	TotalQuestions      int                      `json:"total_questions"`
	AnsweredQuestions   int                      `json:"answered_questions"`
	EvaluatedCategories int                      `json:"evaluated_categories"`
	TotalCategories     int                      `json:"total_categories"`
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
	jobCategoryID := req.JobCategoryID
	storedJobCategoryID := uint(0)
	if s.conversationContextRepo != nil {
		if id, err := s.conversationContextRepo.GetJobCategoryID(req.SessionID); err == nil {
			storedJobCategoryID = id
		}
	}
	if jobCategoryID == 0 {
		jobCategoryID = storedJobCategoryID
	}
	if jobCategoryID != 0 && s.conversationContextRepo != nil && storedJobCategoryID != jobCategoryID {
		if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
			log.Printf("Warning: failed to store job category: %v\n", err)
		}
	}

	jobJustResolved := false
	if jobCategoryID == 0 && s.shouldValidateJobCategory(history) {
		log.Printf("[JobValidation] Validating job category answer: %s\n", req.Message)
		jobValidation, err := s.jobValidator.ValidateJobCategory(ctx, req.Message)
		if err != nil {
			log.Printf("[JobValidation] Error: %v\n", err)
			// エラーでも続行
		} else if jobValidation != nil {
			if jobValidation.IsValid && len(jobValidation.MatchedCategories) > 0 {
				// 明確に職種が特定できた場合
				log.Printf("[JobValidation] Valid job category matched: %d categories\n", len(jobValidation.MatchedCategories))
				jobCategoryID = jobValidation.MatchedCategories[0].ID
				jobJustResolved = true
				if s.conversationContextRepo != nil {
					if err := s.conversationContextRepo.SetJobCategoryID(req.UserID, req.SessionID, jobCategoryID); err != nil {
						log.Printf("Warning: failed to store job category: %v\n", err)
					}
				}
			} else if jobValidation.NeedsClarification && jobValidation.SuggestedQuestion != "" {
				// 職種が曖昧な場合は選択肢を提示
				log.Printf("[JobValidation] Needs clarification, presenting options\n")

				assistantMsg := &models.ChatMessage{
					SessionID: req.SessionID,
					UserID:    req.UserID,
					Role:      "assistant",
					Content:   jobValidation.SuggestedQuestion,
				}
				if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
					log.Printf("Warning: failed to save assistant message: %v\n", err)
				}

				return &ChatResponse{
					Response:          jobValidation.SuggestedQuestion,
					IsComplete:        false,
					TotalQuestions:    15,
					AnsweredQuestions: 0,
				}, nil
			}
		}
	}

	if jobCategoryID != 0 {
		if err := s.completeJobAnalysisPhase(req.UserID, req.SessionID); err != nil {
			log.Printf("Warning: failed to complete job analysis phase: %v\n", err)
		}
	}

	// 2.5. 回答の妥当性チェック（保存後のhistoryを使用）
	handled, response, err := s.checkAnswerValidity(ctx, history, req.Message, req.UserID, req.SessionID)
	if err != nil {
		return nil, err
	}

	// 無効な回答の場合は、ここで処理を終了
	if handled {
		currentPhase, phaseErr := s.getCurrentOrNextPhase(ctx, req.UserID, req.SessionID)
		if phaseErr == nil {
			if err := s.updatePhaseProgress(currentPhase, false); err != nil {
				log.Printf("Warning: failed to update phase progress for invalid answer: %v\n", err)
			}
		}

		validation, err := s.sessionValidationRepo.GetOrCreate(req.SessionID)
		if err != nil {
			log.Printf("Warning: failed to get validation: %v\n", err)
		}

		allPhases, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)

		chatResponse := &ChatResponse{
			Response:          response,
			IsComplete:        false,
			TotalQuestions:    15,
			AnsweredQuestions: len(history) / 2,
			AllPhases:         allPhases,
			CurrentPhase:      currentPhaseInfo,
		}

		if validation != nil {
			chatResponse.InvalidAnswerCount = validation.InvalidAnswerCount
			chatResponse.IsTerminated = validation.IsTerminated

			// 3回目の無効回答の場合は完了フラグを立てる
			if validation.IsTerminated {
				chatResponse.IsComplete = true
			}
		}

		return chatResponse, nil
	}

	// 有効な回答の場合のみ、以降の処理を実行
	// 2.6. 現在のフェーズを取得または開始
	currentPhase, err := s.getCurrentOrNextPhase(ctx, req.UserID, req.SessionID)
	if err != nil {
		// 全フェーズ完了の場合は完了応答を返す
		if err.Error() == "all phases completed" {
			completionMsg := "分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。"

			assistantMsg := &models.ChatMessage{
				SessionID: req.SessionID,
				UserID:    req.UserID,
				Role:      "assistant",
				Content:   completionMsg,
			}
			if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
				log.Printf("Warning: failed to save completion message: %v\n", err)
			}

			allPhases, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)
			return &ChatResponse{
				Response:            completionMsg,
				IsComplete:          true,
				TotalQuestions:      15,
				AnsweredQuestions:   countUserAnswers(history),
				EvaluatedCategories: 10,
				TotalCategories:     10,
				AllPhases:           allPhases,
				CurrentPhase:        currentPhaseInfo,
			}, nil
		}
		return nil, fmt.Errorf("failed to get current phase: %w", err)
	}

	allPhases, err := s.phaseRepo.FindAll()
	if err != nil {
		log.Printf("Warning: failed to get phases: %v\n", err)
	}
	completedProgresses, _ := s.progressRepo.FindByUserAndSession(req.UserID, req.SessionID)
	completedPhaseCount := 0
	phaseByID := make(map[uint]*entity.AnalysisPhase, len(allPhases))
	for i := range allPhases {
		phaseByID[allPhases[i].ID] = &allPhases[i]
	}
	for _, p := range completedProgresses {
		phase := p.Phase
		if phase == nil {
			phase = phaseByID[p.PhaseID]
		}
		if isPhaseComplete(p.ValidAnswers, phase) {
			completedPhaseCount++
		}
	}
	if completedPhaseCount == len(allPhases) && len(allPhases) > 0 {
		completionMsg := "分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。"
		assistantMsg := &models.ChatMessage{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Role:      "assistant",
			Content:   completionMsg,
		}
		if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
			log.Printf("Warning: failed to save completion message: %v\n", err)
		}
		allPhasesInfo, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)
		evaluatedCategoriesCount := 0
		scores, err := s.userWeightScoreRepo.FindByUserAndSession(req.UserID, req.SessionID)
		if err != nil {
			log.Printf("Warning: failed to get scores for completion response: %v\n", err)
		} else {
			seenCategories := make(map[string]bool)
			for _, score := range scores {
				if score.Score != 0 {
					seenCategories[score.WeightCategory] = true
				}
			}
			evaluatedCategoriesCount = len(seenCategories)
		}
		totalMinQuestions := 0
		for _, phase := range allPhases {
			if phase.MaxQuestions > 0 {
				totalMinQuestions += phase.MaxQuestions
			} else {
				totalMinQuestions += phase.MinQuestions
			}
		}
		return &ChatResponse{
			Response:            completionMsg,
			IsComplete:          true,
			TotalQuestions:      totalMinQuestions,
			AnsweredQuestions:   countUserAnswers(history),
			EvaluatedCategories: evaluatedCategoriesCount,
			TotalCategories:     10,
			AllPhases:           allPhasesInfo,
			CurrentPhase:        currentPhaseInfo,
		}, nil
	}

	// 3. ユーザーの回答から重み係数を判定・更新
	// 3. ユーザーの回答から重み係数を判定・更新し、結果に応じてフェーズ進捗を更新
	// スコア更新に成功した場合のみ有効回答としてカウントする
	trimmedAnswer := strings.TrimSpace(req.Message)
	log.Printf("[ProcessChat] Checking if choice answer: '%s' (len=%d)\n", trimmedAnswer, len(trimmedAnswer))
	scoreUpdated := false
	if len(trimmedAnswer) <= 3 && s.isChoiceAnswer(trimmedAnswer) {
		log.Printf("[ProcessChat] Processing as choice answer\n")
		// 選択肢回答の場合は直接スコアを計算
		if err := s.processChoiceAnswer(ctx, req.UserID, req.SessionID, trimmedAnswer, history, jobCategoryID); err != nil {
			log.Printf("Warning: failed to process choice answer: %v\n", err)
		} else {
			scoreUpdated = true
		}
	} else {
		log.Printf("[ProcessChat] Processing as text answer\n")
		// 通常の回答分析
		if err := s.analyzeAndUpdateWeights(ctx, req.UserID, req.SessionID, req.Message, jobCategoryID); err != nil {
			// ログに記録するが、処理は継続
			log.Printf("Warning: failed to update weights: %v\n", err)
		} else {
			scoreUpdated = true
		}
	}

	// フェーズ進捗を更新（スコア更新成功時のみ有効カウント）
	if err := s.updatePhaseProgress(currentPhase, scoreUpdated); err != nil {
		log.Printf("Warning: failed to update phase progress: %v\n", err)
	}

	// 4. 既に聞いた質問を全て収集（重複防止を徹底）
	askedTexts := make(map[string]bool)

	// 4-1. AI生成質問テーブルから取得
	askedQuestions, err := s.aiGeneratedQuestionRepo.FindByUserAndSession(req.UserID, req.SessionID)
	if err != nil {
		log.Printf("Warning: failed to get asked questions: %v\n", err)
		askedQuestions = []models.AIGeneratedQuestion{}
	}
	for _, q := range askedQuestions {
		questionText := normalizeQuestionText(q.QuestionText)
		if questionText == "" {
			questionText = strings.TrimSpace(q.QuestionText)
		}
		if questionText != "" {
			askedTexts[questionText] = true
		}
	}

	// 4-2. チャット履歴からもアシスタントの質問を収集
	for _, msg := range history {
		if msg.Role == "assistant" {
			questionText := normalizeQuestionText(msg.Content)
			if questionText != "" {
				askedTexts[questionText] = true
			}
		}
	}

	log.Printf("Total asked questions for duplicate check: %d\n", len(askedTexts))

	// 5. 現在のスコアを分析して、次に評価すべきカテゴリを決定
	targetLevel := s.getUserTargetLevel(req.UserID)
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(req.UserID, req.SessionID)
	if err != nil {
		log.Printf("Warning: failed to get scores for question selection: %v\n", err)
	}

	// スコア分布を分析
	scoreMap := make(map[string]int)
	evaluatedCategories := make(map[string]bool)
	for _, score := range scores {
		scoreMap[score.WeightCategory] = score.Score
		if score.Score != 0 {
			evaluatedCategories[score.WeightCategory] = true
		}
	}

	// 全カテゴリ（職種に応じて並び順を調整）
	allCategories := s.getCategoryOrder(jobCategoryID)

	// 未評価カテゴリを優先的に選択
	var targetCategory string
	unevaluatedCategories := []string{}
	weaklyEvaluatedCategories := []string{}

	for _, cat := range allCategories {
		score, exists := scoreMap[cat]
		if !exists || score == 0 {
			unevaluatedCategories = append(unevaluatedCategories, cat)
		} else if score > -3 && score < 3 {
			// スコアが-3〜3の範囲は評価が曖昧
			weaklyEvaluatedCategories = append(weaklyEvaluatedCategories, cat)
		}
	}

	if len(unevaluatedCategories) > 0 {
		targetCategory = unevaluatedCategories[0]
		log.Printf("Targeting unevaluated category: %s\n", targetCategory)
	} else if len(weaklyEvaluatedCategories) > 0 {
		targetCategory = weaklyEvaluatedCategories[0]
		log.Printf("Targeting weakly evaluated category: %s (score: %d)\n", targetCategory, scoreMap[targetCategory])
	} else {
		// 全カテゴリ評価済みなら、最もスコアが極端なものを深掘り
		maxAbsScore := 0
		for cat, score := range scoreMap {
			absScore := score
			if absScore < 0 {
				absScore = -absScore
			}
			if absScore > maxAbsScore {
				maxAbsScore = absScore
				targetCategory = cat
			}
		}
		log.Printf("All categories evaluated, deepening strongest: %s (score: %d)\n", targetCategory, scoreMap[targetCategory])
	}

	// 常にまずルールベース質問を試し、なければAIで生成
	var questionWeightID uint
	var aiResponse string

	// 質問生成には最新10件の履歴のみ使用（文脈を保ちつつ、プロンプトを短く）
	recentHistory := history
	if len(history) > 10 {
		recentHistory = history[len(history)-10:]
	}

	// まず、ルールベース質問から選択を試みる
	log.Printf("[RuleBased] Attempting to get predefined question for category: %s\n", targetCategory)
	currentPhaseName := ""
	if currentPhase != nil && currentPhase.Phase != nil {
		currentPhaseName = currentPhase.Phase.PhaseName
	}
	predefinedQ, err := s.tryGetPredefinedQuestion(req.UserID, req.SessionID, targetCategory, req.IndustryID, jobCategoryID, targetLevel, askedTexts, currentPhaseName)

	if err == nil && predefinedQ != nil {
		log.Printf("[RuleBased] Using predefined question (ID: %d) for category: %s\n", predefinedQ.ID, predefinedQ.Category)
		aiResponse = predefinedQ.QuestionText
		questionWeightID = predefinedQ.ID
	} else {
		// ルールベース質問がない場合、AIで生成
		log.Printf("[AI] No predefined question available, generating with AI for category: %s (asked: %d questions)\n", targetCategory, len(askedTexts))
		aiResponse, _, err = s.generateStrategicQuestion(ctx, recentHistory, req.UserID, req.SessionID, scoreMap, allCategories, askedTexts, req.IndustryID, jobCategoryID, targetLevel, currentPhase)
		if err != nil {
			// エラーは致命的にせずフォールバック質問を設定
			log.Printf("Warning: failed to generate question via AI: %v\n", err)
			fallbackQuestion := s.selectFallbackQuestion(targetCategory, jobCategoryID, targetLevel, askedTexts)
			if fallbackQuestion != "" {
				aiResponse = fallbackQuestion
			} else {
				aiResponse = "すみません、質問を生成できませんでした。少し時間をおいてからもう一度お試しください。"
			}
		}
	}
	if currentPhaseName != "" && isTextBasedQuestion(aiResponse) && !shouldForceTextQuestion(recentHistory, currentPhase) {
		if currentPhaseName == "job_analysis" || currentPhaseName == "interest_analysis" || currentPhaseName == "aptitude_analysis" || currentPhaseName == "future_analysis" {
			aiResponse = buildChoiceFallback(aiResponse, currentPhaseName)
		}
	}

	// 5. フェーズベースの完了判定
	// 全フェーズが完了しているかチェック
	completedPhaseCount = 0
	for _, p := range completedProgresses {
		phase := p.Phase
		if phase == nil {
			phase = phaseByID[p.PhaseID]
		}
		if isPhaseComplete(p.ValidAnswers, phase) {
			completedPhaseCount++
		}
	}

	// 質問数を計算（進捗表示用）
	answeredCount := countUserAnswers(history)
	_ = allPhasesReachedMax(completedProgresses, allPhases)

	// 完了判定: 全フェーズが完了していれば終了
	isComplete := completedPhaseCount == len(allPhases)

	log.Printf("Diagnosis progress: %d phases completed out of %d, %d questions asked, %d/10 categories evaluated, complete: %v\n",
		completedPhaseCount, len(allPhases), answeredCount, len(evaluatedCategories), isComplete)

	// 診断完了時のメッセージは追加しない（次の回答時に完了判定する）

	// 6. AIの応答を保存
	// Guard: do not save empty assistant messages
	if strings.TrimSpace(aiResponse) != "" {
		if jobJustResolved {
			aiResponse = "ありがとうございます！それでは、適性診断を始めますね。\n\n" + aiResponse
		}
		if targetLevel == "新卒" && isVerboseQuestion(aiResponse) && isTextBasedQuestion(aiResponse) {
			simple, err := s.simplifyQuestionWithAI(ctx, aiResponse)
			if err != nil || strings.TrimSpace(simple) == "" {
				simple = s.selectFallbackQuestion(targetCategory, jobCategoryID, targetLevel, askedTexts)
			}
			if strings.TrimSpace(simple) == "" {
				simple = simplifyNewGradQuestion(aiResponse)
			}
			aiResponse = simple
		}
		// 新卒向けに表現を調整（全フェーズ共通）
		if targetLevel == "新卒" {
			aiResponse = sanitizeForNewGrad(aiResponse)
		}

		assistantMsg := &models.ChatMessage{
			SessionID:        req.SessionID,
			UserID:           req.UserID,
			Role:             "assistant",
			Content:          aiResponse,
			QuestionWeightID: questionWeightID,
		}
		if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
			log.Printf("Warning: failed to save assistant message: %v\n", err)
			// 続行は可能にする
		}
	} else {
		// フォールバック: 空のAI応答の場合は簡易質問を返す
		log.Printf("Warning: skipped saving empty assistant message for session %s user %d\n", req.SessionID, req.UserID)
		aiResponse = "すみません、質問を生成できませんでした。少し時間をおいてからもう一度お試しください。"
	}

	if isComplete {
		if err := s.ensureEmbeddings(ctx, req.UserID, req.SessionID, jobCategoryID); err != nil {
			log.Printf("Warning: failed to ensure embeddings: %v\n", err)
		}
	}

	// 7. 現在のスコアを取得
	finalScores, err := s.userWeightScoreRepo.FindByUserAndSession(req.UserID, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get scores: %w", err)
	}

	// フェーズ情報を構築
	allPhasesInfo, currentPhaseInfo, _ := s.buildPhaseProgressResponse(req.UserID, req.SessionID)

	// フェーズの質問数合計を計算（最大が無い場合は最小を採用）
	totalMaxQuestions := 0
	for _, phase := range allPhases {
		if phase.MaxQuestions > 0 {
			totalMaxQuestions += phase.MaxQuestions
		} else {
			totalMaxQuestions += phase.MinQuestions
		}
	}

	return &ChatResponse{
		Response:            aiResponse,
		QuestionWeightID:    questionWeightID,
		CurrentScores:       finalScores,
		CurrentPhase:        currentPhaseInfo,
		AllPhases:           allPhasesInfo,
		IsComplete:          isComplete,
		TotalQuestions:      totalMaxQuestions, // 全フェーズの最低質問数合計（最大が無い場合）
		AnsweredQuestions:   answeredCount,
		EvaluatedCategories: len(evaluatedCategories),
		TotalCategories:     10,
	}, nil
}
