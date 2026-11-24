package chat

import (
	"Backend/internal/logsafe"
	"Backend/internal/models"
	"Backend/internal/services/prompts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"slices"
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
		TotalQuestions:      defaultTargetQuestions,
		AnsweredQuestions:   0,
		EvaluatedCategories: 0,
		TotalCategories:     10,
	}, nil
}

// generateStrategicQuestion AIが戦略的に次の質問を生成
// phaseCategories はフェーズごとに出題を許す評価軸。
//
// 全フェーズの和集合が正典10種(domain/valueobject/match.go)を覆っていないと、
// 覆われていない軸はどのフェーズでも出題されず永久に未評価のまま残る。
// マッチングは企業側の10軸と突き合わせるので、そのぶん情報が捨てられる(#1333)。
// phase_categories_test.go がこの網羅を強制する。
var phaseCategories = map[string][]string{
	"job_analysis":      {"技術志向", "創造性志向", "成長志向", "安定志向"},
	"interest_analysis": {"技術志向", "創造性志向", "成長志向", "チャレンジ志向"},
	"aptitude_analysis": {"コミュニケーション力", "チームワーク志向", "リーダーシップ志向", "細部志向"},
	"future_analysis":   {"安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向"},
}

func allowedCategoriesForPhase(phaseName string, all []string) []string {
	if phaseAllowed, ok := phaseCategories[phaseName]; ok && len(phaseAllowed) > 0 {
		return phaseAllowed
	}
	return all
}

func askedCategoryCounts(history []models.ChatMessage) map[string]int {
	counts := map[string]int{}
	for _, msg := range history {
		if msg.Role != "assistant" {
			continue
		}
		cat := strings.TrimSpace(msg.WeightCategory)
		if cat == "" {
			continue
		}
		counts[cat]++
	}
	return counts
}

// pickPhaseTarget は、フェーズの質問枠を先頭の軸が使い切らないように次の軸を選ぶ。
// 出題回数が少ない軸を優先し、同数なら未評価（スコア無しまたは0）、その次に並び順。
// 先頭の未評価をスコアが付くまで繰り返すと、後ろの軸は診断が完了しても一度も聞かれない（SOCAIAGENT-473）。
func pickPhaseTarget(allowed []string, scoreMap map[string]float64, asked map[string]int) (string, string) {
	if len(allowed) == 0 {
		return "", ""
	}
	bestIdx := 0
	bestAsked := int(^uint(0) >> 1)
	bestUnevaluated := false
	for i, cat := range allowed {
		n := asked[cat]
		score, ok := scoreMap[cat]
		unevaluated := !ok || score == 0
		if n < bestAsked || (n == bestAsked && unevaluated && !bestUnevaluated) {
			bestIdx = i
			bestAsked = n
			bestUnevaluated = unevaluated
		}
	}
	cat := allowed[bestIdx]
	if bestUnevaluated {
		return cat, fmt.Sprintf("まだ評価できていない「%s」を評価するため", cat)
	}
	return cat, fmt.Sprintf("「%s」を他の軸と同じ回数まで確認するため", cat)
}

func (s *ChatService) generateStrategicQuestion(ctx context.Context, history []models.ChatMessage, userID uint, sessionID string, scoreMap map[string]float64, allCategories []string, askedTexts map[string]bool, industryID, jobCategoryID uint, targetLevel string, currentPhase *models.UserAnalysisProgress, asked map[string]int) (string, uint, string, error) {
	ctx, resolvedVersion := prompts.WithPromptVersion(ctx, prompts.StageQuestionGeneration)

	// 狙った軸は呼び出し元に返す。ここはフェーズで絞った allowedCategories から
	// 選ぶので、呼び出し元が allCategories から決めた軸とは一致しないことがある。
	// 採点側が質問文から軸を推測し直すと、キーワードに当たらない質問が既定の
	// 技術志向に落ちて狙った軸が未評価のまま残る(#1333)。
	// 会話履歴を構築
	historyText := buildHistoryText(history)

	// 既に聞いた質問のリスト（重複防止を徹底）
	askedQuestionsText := buildAskedQuestionsText(askedTexts)

	phaseName := ""
	if currentPhase != nil && currentPhase.Phase != nil {
		phaseName = currentPhase.Phase.PhaseName
	}
	allowedCategories := allowedCategoriesForPhase(phaseName, allCategories)

	// スコア状況の分析（フェーズ対象カテゴリのみ）
	scoreAnalysis := buildScoreAnalysis(allowedCategories, scoreMap)

	// 職種名と業界名を取得
	jobCategoryName := s.jobCategoryNameForPrompt(ctx, jobCategoryID)

	// 出題回数はセッション全体で数える。プロンプト用の history は直近だけなので、
	// ここで数えると前のフェーズの出題が消えて同じ軸に戻る。
	if asked == nil {
		asked = askedCategoryCounts(history)
	}
	targetCategory, questionPurpose := pickPhaseTarget(allowedCategories, scoreMap, asked)

	// フェーズ情報を追加
	phaseContext := buildPhaseContext(currentPhase)
	phaseContext += s.consultIntentContext(sessionID)
	// phaseName はフェーズカテゴリ選定で取得済み
	forceTextQuestion := shouldForceTextQuestion(history, currentPhase)
	choiceGuidance := buildChoiceGuidance(phaseName, forceTextQuestion)

	if strings.TrimSpace(targetLevel) == "" {
		targetLevel = "新卒"
	}

	requiresChoice := currentPhase != nil && !forceTextQuestion && (phaseName == "" || phaseName == "job_analysis" || phaseName == "interest_analysis" || phaseName == "aptitude_analysis" || phaseName == "future_analysis")

	description := categoryDescriptionFor(targetLevel, targetCategory)

	prompt := prompts.BuildStrategicQuestionPromptWithPhase(
		targetLevel, phaseName, phaseContext, choiceGuidance,
		historyText, scoreAnalysis, askedQuestionsText,
		questionPurpose, targetCategory, description,
		jobCategoryName, industryID, jobCategoryID,
	)

	questionText, err := s.questionAICall(ctx, prompt)
	if err != nil {
		return "", 0, "", err
	}

	// 質問文をクリーンアップ
	questionText = cleanGeneratedQuestion(questionText)

	promptVersion := resolvedVersion
	// フォールバック: AIが空を返した場合は簡易質問を使用する
	if questionText == "" {
		fallbackQuestion := s.selectFallbackQuestion(ctx, targetCategory, jobCategoryID, targetLevel, askedTexts)
		if fallbackQuestion != "" {
			questionText = fallbackQuestion
		} else {
			questionText = "すみません、質問を生成できませんでした。少し時間をおいてからもう一度お試しください。"
		}
		promptVersion = ""
	}

	// 選択肢必須フェーズで選択肢がない場合は再生成
	// 代わりの質問を AI が作り直したら、保存するのは AI の文なので版を残す(#2573)。
	if requiresChoice && isTextBasedQuestion(questionText) {
		var rewritten bool
		questionText, rewritten = s.ensureChoiceQuestion(ctx, questionText, targetCategory)
		if rewritten {
			promptVersion = resolvedVersion
		}
	}

	// 重複チェック（完全一致および類似度チェック）を最大3回まで試行
	questionText, rewritten, err := s.regenerateIfDuplicate(ctx, questionText, askedTexts, targetCategory)
	if err != nil {
		return "", 0, "", err
	}
	if rewritten {
		promptVersion = resolvedVersion
	}

	// AI生成質問をデータベースに保存（空文字は保存しない）
	questionText = strings.TrimSpace(questionText)
	if questionText == "" {
		log.Printf("Warning: AI generated empty question even after fallback, not saving. user=%d session=%s\n", userID, sessionID)
		return "", 0, "", fmt.Errorf("ai returned empty question")
	}

	aiGenQuestion := &models.AIGeneratedQuestion{
		UserID:        userID,
		SessionID:     sessionID,
		TemplateID:    nil, // AI生成の場合はNULL
		QuestionText:  questionText,
		Weight:        7, // 戦略的質問は重み高め
		IsAnswered:    false,
		PromptVersion: promptVersion,
		ContextData:   buildQuestionContextData(targetCategory, questionPurpose),
	}

	if err := s.aiGeneratedQuestionRepo.Create(aiGenQuestion); err != nil {
		return "", 0, "", fmt.Errorf("failed to save AI generated question: %w", err)
	}

	return questionText, aiGenQuestion.ID, targetCategory, nil
}

// buildQuestionContextData は保存する質問の文脈（狙った軸と目的）を JSON にする。
// 値に引用符などが入っても壊れないよう encoding/json で組み立てる(#2573)。
func buildQuestionContextData(targetCategory, purpose string) string {
	b, err := json.Marshal(struct {
		TargetCategory string `json:"target_category"`
		Purpose        string `json:"purpose"`
	}{targetCategory, purpose})
	if err != nil {
		// 文字列だけの構造体なので失敗しない
		return "{}"
	}
	return string(b)
}

// sortedAskedTexts は既に聞いた質問を決まった順（文字列順）に並べる。
// map をそのまま回すとプロンプトの並びが毎回変わる(#2573)。
func sortedAskedTexts(askedTexts map[string]bool) []string {
	return slices.Sorted(maps.Keys(askedTexts))
}

// buildHistoryText は会話履歴をプロンプト用の「role: 本文」の行に並べる。
func buildHistoryText(history []models.ChatMessage) string {
	historyText := ""
	for _, msg := range history {
		historyText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}
	return historyText
}

// buildAskedQuestionsText は既に聞いた質問の一覧（重複させないための指示つき）を作る。
func buildAskedQuestionsText(askedTexts map[string]bool) string {
	askedQuestionsText := "\n## 【重要】既に聞いた質問（絶対に重複させないこと）\n"
	if len(askedTexts) == 0 {
		askedQuestionsText += "（まだ質問していません）\n"
	} else {
		questionCount := 0
		for _, text := range sortedAskedTexts(askedTexts) {
			questionCount++
			askedQuestionsText += fmt.Sprintf("%d. %s\n", questionCount, text)
		}
		askedQuestionsText += fmt.Sprintf("\n**上記%d個の質問と類似・重複する質問は絶対に生成しないでください**\n", questionCount)
	}
	return askedQuestionsText
}

// buildScoreAnalysis はフェーズ対象カテゴリの評価状況（点数か未評価）を並べる。
func buildScoreAnalysis(allowedCategories []string, scoreMap map[string]float64) string {
	scoreAnalysis := "## 現在の評価状況\n"
	for _, cat := range allowedCategories {
		score, exists := scoreMap[cat]
		if exists && score != 0 {
			scoreAnalysis += fmt.Sprintf("- %s: %.0f点\n", cat, score)
		} else {
			scoreAnalysis += fmt.Sprintf("- %s: 未評価\n", cat)
		}
	}
	return scoreAnalysis
}

// jobCategoryNameForPrompt はプロンプトに入れる職種名。未指定・取得失敗は「指定なし」。
func (s *ChatService) jobCategoryNameForPrompt(ctx context.Context, jobCategoryID uint) string {
	jobCategoryName := "指定なし"
	if jobCategoryID != 0 {
		if jc, err := s.jobCategoryRepo.FindByID(ctx, jobCategoryID); err == nil && jc != nil {
			jobCategoryName = jc.Name
		}
	}
	return jobCategoryName
}

var categoryDescriptions = map[string]string{
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

var categoryDescriptionsMid = map[string]string{
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

// categoryDescriptionFor は対象カテゴリの説明。中途は中途向けの説明を使う。
func categoryDescriptionFor(targetLevel, category string) string {
	if targetLevel == "中途" {
		return categoryDescriptionsMid[category]
	}
	return categoryDescriptions[category]
}

// buildPhaseContext は現在の分析フェーズの説明。フェーズが無ければ空文字。
func buildPhaseContext(currentPhase *models.UserAnalysisProgress) string {
	if currentPhase == nil || currentPhase.Phase == nil {
		return ""
	}
	if currentPhase.Phase.MaxQuestions > 0 {
		return fmt.Sprintf(`
## 現在の分析フェーズ: %s
%s
このフェーズでは%dつ〜%dつの質問を行います。現在%d個目の質問です。
フェーズの目的に沿った質問を生成してください。
`, currentPhase.Phase.DisplayName, currentPhase.Phase.Description,
			currentPhase.Phase.MinQuestions, currentPhase.Phase.MaxQuestions,
			currentPhase.QuestionsAsked+1)
	}
	return fmt.Sprintf(`
## 現在の分析フェーズ: %s
%s
このフェーズでは最低%dつの質問を行います。現在%d個目の質問です。
フェーズの目的に沿った質問を生成してください。
`, currentPhase.Phase.DisplayName, currentPhase.Phase.Description,
		currentPhase.Phase.MinQuestions,
		currentPhase.QuestionsAsked+1)
}

// buildChoiceGuidance は質問形式（選択肢か自由記述か）の方針。該当が無ければ空文字。
func buildChoiceGuidance(phaseName string, forceTextQuestion bool) string {
	choiceGuidance := ""
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
	return choiceGuidance
}

// cleanGeneratedQuestion は AI が返した質問文の前後の空白と引用符を落とす。
func cleanGeneratedQuestion(text string) string {
	text = strings.TrimSpace(text)
	return strings.Trim(text, `"「」`)
}

// ensureChoiceQuestion は選択肢の無い質問を最大2回まで AI に作り直させる。
// それでも選択肢が無ければ、評価軸ごとの決まった選択肢で補う。
// 作り直しの呼び出しに失敗したときは、その時点の質問文で打ち切る（エラーは返さない）。
// rewritten は AI が作り直した文を採用したかどうか。
func (s *ChatService) ensureChoiceQuestion(ctx context.Context, questionText, targetCategory string) (string, bool) {
	rewritten := false
	for attempt := 0; attempt < 2; attempt++ {
		choicePrompt := fmt.Sprintf(`以下の質問は選択肢が不足しています。
"%s"

必ず4〜5個の選択肢を「A)」「B)」「C)」「D)」「E)」または「1)」「2)」「3)」「4)」「5)」形式で改行区切りで列挙し、最後に「その他（自由記述）」を含めてください。

質問文は1つのみ。説明は不要です。質問文の後に選択肢を列挙してください。`, questionText)

		regenerated, err := s.questionAICall(ctx, choicePrompt)
		if err != nil {
			break
		}
		regenerated = cleanGeneratedQuestion(regenerated)
		if regenerated != "" {
			questionText = regenerated
			rewritten = true
		}
		if !isTextBasedQuestion(questionText) {
			break
		}
	}
	if isTextBasedQuestion(questionText) {
		questionText = buildChoiceFallback(questionText, targetCategory)
	}
	return questionText, rewritten
}

// findDuplicateQuestion は既に聞いた質問との完全一致・類似を調べ、重複なら理由を返す。
func findDuplicateQuestion(questionText string, askedTexts map[string]bool) (bool, string) {
	// 完全一致チェック
	if askedTexts[questionText] {
		return true, fmt.Sprintf("完全一致: %s", questionText)
	}
	// 類似度チェック
	for _, askedQ := range sortedAskedTexts(askedTexts) {
		similarity := calculateSimilarity(questionText, askedQ)
		if similarity > 0.6 { // 閾値を0.6に下げて、より厳格に
			return true, fmt.Sprintf("類似度%.2f: %s", similarity, askedQ)
		}
	}
	return false, ""
}

// regenerateIfDuplicate は重複している間、最大3回まで質問を作り直させる。
// 最後の試行で重複してもそのまま使う（無限ループ防止）。AI 呼び出しの失敗はそのまま返す。
// rewritten は AI が作り直した文に置き換えたかどうか。
func (s *ChatService) regenerateIfDuplicate(ctx context.Context, questionText string, askedTexts map[string]bool, targetCategory string) (string, bool, error) {
	rewritten := false
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		isDuplicate, duplicateReason := findDuplicateQuestion(questionText, askedTexts)
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
				for _, q := range sortedAskedTexts(askedTexts) {
					count++
					list += fmt.Sprintf("%d. %s\n", count, q)
				}
				return list
			}(),
			targetCategory)

		var err error
		questionText, err = s.questionAICall(ctx, retryPrompt)
		if err != nil {
			return "", false, err
		}
		questionText = cleanGeneratedQuestion(questionText)
		rewritten = true

		// 最後の試行で重複してもそのまま使用（無限ループ防止）
		if attempt == maxRetries-1 {
			log.Printf("Max retries reached, using question anyway: chars=%d\n", logsafe.Chars(questionText))
		}
	}
	return questionText, rewritten, nil
}
