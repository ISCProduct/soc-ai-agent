package services

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"fmt"
	"log"
	"strings"
)

type processQuestionInput struct {
	req                 ChatRequest
	history             []models.ChatMessage
	jobCategoryID       uint
	jobJustResolved     bool
	currentPhase        *entity.UserAnalysisProgress
	allPhases           []entity.AnalysisPhase
	completedProgresses []entity.UserAnalysisProgress
	phaseByID           map[uint]*entity.AnalysisPhase
}

func (s *ChatService) processAnswerAndNextQuestion(ctx context.Context, input processQuestionInput) (*ChatResponse, error) {
	req := input.req
	history := input.history
	jobCategoryID := input.jobCategoryID
	jobJustResolved := input.jobJustResolved
	currentPhase := input.currentPhase
	allPhases := input.allPhases
	completedProgresses := input.completedProgresses
	phaseByID := input.phaseByID

	// 3. ユーザーの回答から重み係数を判定・更新し、回答品質に応じてフェーズ進捗を更新
	// isQualityAnswer: スキップフレーズ・極短回答・スコア0でなければ true（進捗カウント対象）
	trimmedAnswer := strings.TrimSpace(req.Message)
	lastAssistantQuestion := ""
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastAssistantQuestion = history[i].Content
			break
		}
	}
	resolved := ResolveChoiceAnswer(lastAssistantQuestion, trimmedAnswer)
	log.Printf("[ProcessChat] Checking answer: raw=%q resolved_choice=%v letter=%q free_text=%v\n",
		trimmedAnswer, resolved.IsChoice, resolved.Letter, resolved.IsFreeText)
	isQualityAnswer := false
	if resolved.IsChoice && s.isChoiceAnswer(resolved.Letter) {
		log.Printf("[ProcessChat] Processing as choice answer\n")
		var err error
		isQualityAnswer, err = s.processChoiceAnswer(ctx, req.UserID, req.SessionID, resolved.Letter, history, jobCategoryID)
		if err != nil {
			log.Printf("Warning: failed to process choice answer: %v\n", err)
		}
	} else {
		log.Printf("[ProcessChat] Processing as text answer\n")
		var err error
		isQualityAnswer, err = s.analyzeAndUpdateWeights(ctx, req.UserID, req.SessionID, resolved.Text, jobCategoryID)
		if err != nil {
			log.Printf("Warning: failed to update weights: %v\n", err)
		}
	}

	// 品質回答のみ valid_answers をインクリメントして進捗を進める
	if err := s.updatePhaseProgress(currentPhase, isQualityAnswer); err != nil {
		log.Printf("Warning: failed to update phase progress: %v\n", err)
	}

	// 4. 既に聞いた質問を全て収集（重複防止を徹底）
	askedTexts := s.collectAskedTexts(req.UserID, req.SessionID, history)

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
	completedPhaseCount := 0
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

	// 完了している場合は要約を生成して応答に含める
	var sessionSummary *SessionSummary
	if isComplete {
		summary, _ := s.buildAndSaveSessionSummary(context.Background(), req.UserID, req.SessionID)
		sessionSummary = summary
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
		Summary:             sessionSummary,
		JobCategoryID:       jobCategoryID,
	}, nil
}

func (s *ChatService) collectAskedTexts(userID uint, sessionID string, history []models.ChatMessage) map[string]bool {
	askedTexts := make(map[string]bool)

	// 4-1. AI生成質問テーブルから取得
	askedQuestions, err := s.aiGeneratedQuestionRepo.FindByUserAndSession(userID, sessionID)
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

	return askedTexts
}

func mergeAskedTexts(history []models.ChatMessage, aiQuestions []models.AIGeneratedQuestion) map[string]bool {
	askedTexts := make(map[string]bool)

	for _, q := range aiQuestions {
		questionText := normalizeQuestionText(q.QuestionText)
		if questionText == "" {
			questionText = strings.TrimSpace(q.QuestionText)
		}
		if questionText != "" {
			askedTexts[questionText] = true
		}
	}

	for _, msg := range history {
		if msg.Role == "assistant" {
			questionText := normalizeQuestionText(msg.Content)
			if questionText != "" {
				askedTexts[questionText] = true
			}
		}
	}

	return askedTexts
}
