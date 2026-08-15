package chat

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/prompts"
	"context"
	"fmt"
	"log"
	"strings"
)

// handleSessionStart セッション開始時の初回質問を生成
func (s *ChatService) handleSessionStart(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	log.Printf("Starting new session: %s\n", req.SessionID)

	// ユーザー情報を取得
	user, err := s.userRepo.GetUserByID(req.UserID)
	userName := "あなた"
	if err == nil && user != nil && user.Name != "" {
		userName = user.Name
	}

	// 職種選択の質問を生成
	jobQuestion, err := s.jobValidator.GenerateJobSelectionQuestion(ctx)
	if err != nil {
		// エラー時のフォールバック
		jobQuestion = `初めまして！あなたの適性診断をサポートします。

まず、どの職種に興味がありますか？以下から選んでください：

1. エンジニア（プログラミング、開発）
2. 営業（顧客対応、提案）
3. マーケティング（企画、分析）
4. 人事（採用、育成）
5. その他・まだ決めていない

番号で答えても、職種名で答えても構いません。`
	} else {
		jobQuestion = fmt.Sprintf("初めまして、%sさん！あなたの適性診断をサポートします。\n\n%s", userName, jobQuestion)
	}

	response := jobQuestion

	// 初回メッセージを保存
	assistantMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      "assistant",
		Content:   response,
	}
	if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
		return nil, fmt.Errorf("failed to save initial message: %w", err)
	}

	return &ChatResponse{
		Response:            response,
		IsComplete:          false,
		TotalQuestions:      15,
		AnsweredQuestions:   0,
		EvaluatedCategories: 0,
		TotalCategories:     10,
	}, nil
}

// generateStrategicQuestion AIが戦略的に次の質問を生成
func (s *ChatService) generateStrategicQuestion(ctx context.Context, history []models.ChatMessage, userID uint, sessionID string, scoreMap map[string]int, allCategories []string, askedTexts map[string]bool, industryID, jobCategoryID uint, targetLevel string, currentPhase *entity.UserAnalysisProgress) (string, uint, error) {
	// 会話履歴を構築
	historyText := ""
	for _, msg := range history {
		historyText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	// 既に聞いた質問のリスト（重複防止を徹底）
	askedQuestionsText := "\n## 【重要】既に聞いた質問（絶対に重複させないこと）\n"
	if len(askedTexts) == 0 {
		askedQuestionsText += "（まだ質問していません）\n"
	} else {
		questionCount := 0
		for text := range askedTexts {
			questionCount++
			askedQuestionsText += fmt.Sprintf("%d. %s\n", questionCount, text)
		}
		askedQuestionsText += fmt.Sprintf("\n**上記%d個の質問と類似・重複する質問は絶対に生成しないでください**\n", questionCount)
	}

	phaseCategories := map[string][]string{
		"job_analysis":      {"技術志向", "創造性志向", "成長志向", "安定志向"},
		"interest_analysis": {"技術志向", "創造性志向", "成長志向", "チャレンジ志向"},
		"aptitude_analysis": {"コミュニケーション力", "チームワーク志向", "リーダーシップ志向", "細部志向"},
		"future_analysis":   {"安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向"},
	}

	allowedCategories := allCategories
	phaseName := ""
	if currentPhase != nil && currentPhase.Phase != nil {
		phaseName = currentPhase.Phase.PhaseName
		if phaseAllowed, ok := phaseCategories[phaseName]; ok && len(phaseAllowed) > 0 {
			allowedCategories = phaseAllowed
		}
	}

	// スコア状況の分析（フェーズ対象カテゴリのみ）
	scoreAnalysis := "## 現在の評価状況\n"
	evaluatedCategories := []string{}
	unevaluatedCategories := []string{}

	for _, cat := range allowedCategories {
		score, exists := scoreMap[cat]
		if exists && score != 0 {
			scoreAnalysis += fmt.Sprintf("- %s: %d点\n", cat, score)
			evaluatedCategories = append(evaluatedCategories, cat)
		} else {
			unevaluatedCategories = append(unevaluatedCategories, cat)
		}
	}

	// 職種名と業界名を取得
	jobCategoryName := "指定なし"
	if jobCategoryID != 0 {
		if jc, err := s.jobCategoryRepo.FindByID(jobCategoryID); err == nil && jc != nil {
			jobCategoryName = jc.Name
		}
	}

	// 企業選定に必要な情報を特定
	var targetCategory string
	var questionPurpose string

	if len(unevaluatedCategories) > 0 {
		// 未評価カテゴリがあれば優先
		targetCategory = unevaluatedCategories[0]
		questionPurpose = fmt.Sprintf("まだ評価できていない「%s」を評価するため", targetCategory)
	} else {
		// 全カテゴリ評価済みなら、スコアが中途半端なものを深掘り
		targetCategory = ""
		for _, cat := range allowedCategories {
			score := scoreMap[cat]
			if score > -3 && score < 3 {
				targetCategory = cat
				questionPurpose = fmt.Sprintf("評価が曖昧な「%s」をより明確に判定するため", cat)
				break
			}
		}

		if targetCategory == "" {
			// 最もスコアが高いカテゴリを深掘り
			highestScore := -100
			for _, cat := range allowedCategories {
				score := scoreMap[cat]
				if score > highestScore {
					highestScore = score
					targetCategory = cat
				}
			}
			questionPurpose = fmt.Sprintf("強みである「%s」をさらに深く評価し、最適な企業を絞り込むため", targetCategory)
		}
	}

	categoryDescriptions := map[string]string{
		"技術志向":       "技術やデジタル活用への興味、学習経験（授業、趣味、独学）→ 技術主導企業か事業主導企業か",
		"コミュニケーション力": "対話力、説明力、プレゼン経験（授業発表、サークル）→ チーム重視企業か個人裁量企業か",
		"リーダーシップ志向":  "主導性、提案力、まとめ役経験（グループワーク、サークル）→ マネジメント志向かスペシャリスト志向か",
		"チームワーク志向":   "協力、役割認識、グループ活動経験（授業、サークル、バイト）→ 大規模チーム企業か少数精鋭企業か",
		"創造性志向":      "独創性、アイデア発想、工夫した経験（課題、趣味）→ スタートアップか大企業か",
		"安定志向":       "長期的キャリア観、安定性重視 → 大手企業かベンチャーか",
		"成長志向":       "学習意欲、自己成長、新しい挑戦（資格、自主学習）→ 教育重視企業か実践重視企業か",
		"チャレンジ志向":    "困難への挑戦、失敗を恐れない姿勢 → 挑戦推奨文化か安定志向文化か",
		"細部志向":       "丁寧さ、正確性、品質へのこだわり → 品質重視企業かスピード重視企業か",
		"ワークライフバランス": "仕事と私生活のバランス観 → ワークライフバランス重視企業か成果主義企業か",
	}
	categoryDescriptionsMid := map[string]string{
		"技術志向":       "技術への興味、業務での技術活用や改善経験 → 技術主導企業か事業主導企業か",
		"コミュニケーション力": "関係者との調整、説明力、合意形成の経験 → チーム重視企業か個人裁量企業か",
		"リーダーシップ志向":  "意思決定、主導性、チームや案件の推進経験 → マネジメント志向かスペシャリスト志向か",
		"チームワーク志向":   "協力、役割認識、チームでの成果創出経験 → 大規模チーム企業か少数精鋭企業か",
		"創造性志向":      "改善提案、業務の工夫、新しいアプローチ → スタートアップか大企業か",
		"安定志向":       "長期的キャリア観、安定性重視 → 大手企業かベンチャーか",
		"成長志向":       "学習意欲、自己成長、新しい挑戦 → 教育重視企業か実践重視企業か",
		"チャレンジ志向":    "困難への挑戦、失敗を恐れない姿勢 → 挑戦推奨文化か安定志向文化か",
		"細部志向":       "丁寧さ、正確性、品質へのこだわり → 品質重視企業かスピード重視企業か",
		"ワークライフバランス": "仕事と私生活のバランス観 → ワークライフバランス重視企業か成果主義企業か",
	}

	// フェーズ情報を追加
	phaseContext := ""
	if currentPhase != nil && currentPhase.Phase != nil {
		if currentPhase.Phase.MaxQuestions > 0 {
			phaseContext = fmt.Sprintf(`
## 現在の分析フェーズ: %s
%s
このフェーズでは%dつ〜%dつの質問を行います。現在%d個目の質問です。
フェーズの目的に沿った質問を生成してください。
`, currentPhase.Phase.DisplayName, currentPhase.Phase.Description,
				currentPhase.Phase.MinQuestions, currentPhase.Phase.MaxQuestions,
				currentPhase.QuestionsAsked+1)
		} else {
			phaseContext = fmt.Sprintf(`
## 現在の分析フェーズ: %s
%s
このフェーズでは最低%dつの質問を行います。現在%d個目の質問です。
フェーズの目的に沿った質問を生成してください。
`, currentPhase.Phase.DisplayName, currentPhase.Phase.Description,
				currentPhase.Phase.MinQuestions,
				currentPhase.QuestionsAsked+1)
		}
	}
	choiceGuidance := ""
	// phaseName はフェーズカテゴリ選定で取得済み
	forceTextQuestion := shouldForceTextQuestion(history, currentPhase)
	if phaseName != "" {
		switch phaseName {
		case "job_analysis":
			choiceGuidance = "- 職種分析では選択肢中心で質問を構成する\n- 4〜5択で興味や方向性を選ばせ、最後に「その他（自由記述）」を用意する\n- 選択肢は必ず「A)」「B)」または「1)」「2)」形式で改行区切りで列挙する\n- 出力は『質問文 + 選択肢列挙』の形式とし、選択肢がない質問は不可\n- 文章でないと判定できない場合のみ自由記述にする（その場合も「その他（自由記述）」として選択肢に含める）"
		case "interest_analysis":
			choiceGuidance = "- 興味分析では選択肢中心で質問を構成する\n- 可能な限り4〜5択で提示し、最後に「その他（自由記述）」を用意する\n- 選択肢は必ず「A)」「B)」または「1)」「2)」形式で改行区切りで列挙する\n- 出力は『質問文 + 選択肢列挙』の形式とし、選択肢がない質問は不可\n- 文章必須の深掘りが必要な場合のみ自由記述にする（その場合も「その他（自由記述）」として選択肢に含める）"
		case "aptitude_analysis":
			choiceGuidance = "- 適性分析では選択肢中心で質問を構成する\n- 4〜5択で具体的な行動や傾向を選ばせる\n- 選択肢は必ず「A)」「B)」または「1)」「2)」形式で改行区切りで列挙する\n- 出力は『質問文 + 選択肢列挙』の形式とし、選択肢がない質問は不可\n- 文章でないと判定できない場合のみ自由記述にする（その場合も「その他（自由記述）」として選択肢に含める）"
		case "future_analysis":
			choiceGuidance = "- 将来分析（待遇・働き方の希望を含む）では選択肢中心で質問を構成する\n- 4〜5択で希望や優先順位を選ばせ、最後に「その他（自由記述）」を用意する\n- 選択肢は必ず「A)」「B)」または「1)」「2)」形式で改行区切りで列挙する\n- 出力は『質問文 + 選択肢列挙』の形式とし、選択肢がない質問は不可\n- 理由や背景が必要な場合のみ自由記述にする（その場合も「その他（自由記述）」として選択肢に含める）"
		}
	}
	if forceTextQuestion {
		choiceGuidance = "- このフェーズでは最低限の自由記述質問が必要です\n- 今回は必ず自由記述で質問を作成する\n- 選択肢は出さない"
	}
	if choiceGuidance != "" {
		choiceGuidance = fmt.Sprintf("## 質問形式の方針\n%s\n", choiceGuidance)
	}

	if strings.TrimSpace(targetLevel) == "" {
		targetLevel = "新卒"
	}

	requiresChoice := currentPhase != nil && !forceTextQuestion && (phaseName == "" || phaseName == "job_analysis" || phaseName == "interest_analysis" || phaseName == "aptitude_analysis" || phaseName == "future_analysis")

	description := categoryDescriptions[targetCategory]
	if targetLevel == "中途" {
		description = categoryDescriptionsMid[targetCategory]
	}

	prompt := prompts.BuildStrategicQuestionPromptWithPhase(
		targetLevel, phaseName, phaseContext, choiceGuidance,
		historyText, scoreAnalysis, askedQuestionsText,
		questionPurpose, targetCategory, description,
		jobCategoryName, industryID, jobCategoryID,
	)

	questionText, err := s.aiCallWithRetries(ctx, prompt)
	if err != nil {
		return "", 0, err
	}

	// 質問文をクリーンアップ
	questionText = strings.TrimSpace(questionText)
	questionText = strings.Trim(questionText, `"「」`)

	// フォールバック: AIが空を返した場合は簡易質問を使用する
	if questionText == "" {
		fallbackQuestion := s.selectFallbackQuestion(targetCategory, jobCategoryID, targetLevel, askedTexts)
		if fallbackQuestion != "" {
			questionText = fallbackQuestion
		} else {
			questionText = "すみません、質問を生成できませんでした。少し時間をおいてからもう一度お試しください。"
		}
	}

	// 選択肢必須フェーズで選択肢がない場合は再生成
	if requiresChoice && isTextBasedQuestion(questionText) {
		for attempt := 0; attempt < 2; attempt++ {
			choicePrompt := fmt.Sprintf(`以下の質問は選択肢が不足しています。
"%s"

必ず4〜5個の選択肢を「A)」「B)」「C)」「D)」「E)」または「1)」「2)」「3)」「4)」「5)」形式で改行区切りで列挙し、最後に「その他（自由記述）」を含めてください。

質問文は1つのみ。説明は不要です。質問文の後に選択肢を列挙してください。`, questionText)

			regenerated, err := s.aiCallWithRetries(ctx, choicePrompt)
			if err != nil {
				break
			}
			regenerated = strings.TrimSpace(regenerated)
			regenerated = strings.Trim(regenerated, `"「」`)
			if regenerated != "" {
				questionText = regenerated
			}
			if !isTextBasedQuestion(questionText) {
				break
			}
		}
		if isTextBasedQuestion(questionText) {
			questionText = buildChoiceFallback(questionText, phaseName)
		}
	}

	// 重複チェック（完全一致および類似度チェック）を最大3回まで試行
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		isDuplicate := false
		duplicateReason := ""

		// 完全一致チェック
		if askedTexts[questionText] {
			isDuplicate = true
			duplicateReason = fmt.Sprintf("完全一致: %s", questionText)
		} else {
			// 類似度チェック
			for askedQ := range askedTexts {
				similarity := calculateSimilarity(questionText, askedQ)
				if similarity > 0.6 { // 閾値を0.6に下げて、より厳格に
					isDuplicate = true
					duplicateReason = fmt.Sprintf("類似度%.2f: %s", similarity, askedQ)
					break
				}
			}
		}

		if !isDuplicate {
			break // 重複なし、使用可能
		}

		log.Printf("Retry %d: Duplicate detected (%s)\n", attempt+1, duplicateReason)

		// 再生成プロンプト
		retryPrompt := fmt.Sprintf(`以下の質問は既に聞いているか類似しています：
"%s"

既に聞いた全ての質問：
%s

これらと完全に異なる新しい質問を生成してください。
対象カテゴリ: %s
**質問のみ**を返してください。説明は不要です。`,
			questionText,
			func() string {
				var list string
				count := 0
				for q := range askedTexts {
					count++
					list += fmt.Sprintf("%d. %s\n", count, q)
				}
				return list
			}(),
			targetCategory)

		questionText, err = s.aiCallWithRetries(ctx, retryPrompt)
		if err != nil {
			return "", 0, err
		}
		questionText = strings.TrimSpace(questionText)
		questionText = strings.Trim(questionText, `"「」`)

		// 最後の試行で重複してもそのまま使用（無限ループ防止）
		if attempt == maxRetries-1 {
			log.Printf("Max retries reached, using question anyway: %s\n", questionText)
		}
	}

	// AI生成質問をデータベースに保存（空文字は保存しない）
	questionText = strings.TrimSpace(questionText)
	if questionText == "" {
		log.Printf("Warning: AI generated empty question even after fallback, not saving. user=%d session=%s\n", userID, sessionID)
		return "", 0, fmt.Errorf("ai returned empty question")
	}

	aiGenQuestion := &models.AIGeneratedQuestion{
		UserID:       userID,
		SessionID:    sessionID,
		TemplateID:   nil, // AI生成の場合はNULL
		QuestionText: questionText,
		Weight:       7, // 戦略的質問は重み高め
		IsAnswered:   false,
		ContextData:  fmt.Sprintf(`{"target_category": "%s", "purpose": "%s"}`, targetCategory, questionPurpose),
	}

	if err := s.aiGeneratedQuestionRepo.Create(aiGenQuestion); err != nil {
		return "", 0, fmt.Errorf("failed to save AI generated question: %w", err)
	}

	return questionText, aiGenQuestion.ID, nil
}
