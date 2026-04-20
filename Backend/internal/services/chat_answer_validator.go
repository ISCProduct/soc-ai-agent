package services

import (
	"Backend/internal/models"
	"Backend/internal/services/prompts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// checkAnswerValidity: 直近の assistant メッセージが質問かを判定し、ユーザー入力がその質問に対する有効な回答かを判定する。
// 無効な場合はアシスタントの「書かれた内容にはお答えできません」メッセージを保存して true を返す。
// 3回連続で無効な場合はセッションを強制終了する。
// 戻り値: handled(bool) - true の場合は処理を終了してよい、response(string) - 保存したアシスタント応答（ある場合）、error
func (s *ChatService) checkAnswerValidity(ctx context.Context, history []models.ChatMessage, userMessage string, userID uint, sessionID string) (bool, string, error) {
	// 直近の assistant メッセージを探す
	var lastAssistant *models.ChatMessage
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			lastAssistant = &history[i]
			break
		}
	}

	// アシスタントメッセージがない場合、またはそれが質問でない場合
	// → これは初回や説明メッセージの直後なので、職種に関する回答を期待する
	var questionText string
	if lastAssistant == nil {
		// 履歴がない場合は、初回の職種選択を期待
		questionText = "どのようなIT職種に興味がありますか？"
	} else if !isQuestion(lastAssistant.Content) {
		// 質問ではない場合（説明文など）も、職種に関する回答を期待
		questionText = "IT業界のどのような職種に興味がありますか？"
	} else {
		// 通常の質問の場合
		questionText = lastAssistant.Content
	}

	// ユーザー回答が質問に対する答えかどうか判定
	isValid, err := s.validateAnswerRelevance(ctx, questionText, userMessage)
	if err != nil {
		// AI判定エラー時は基本的な検証のみ
		log.Printf("[Validation] AI validation failed: %v, using basic validation\n", err)
		isValid = isLikelyAnswer(userMessage, questionText)
		log.Printf("[Validation] Basic validation result: %v for message: %s\n", isValid, userMessage)
	} else {
		log.Printf("[Validation] AI validation result: %v for message: %s\n", isValid, userMessage)
	}

	if isValid {
		// 有効な回答と判断 -> カウントをリセットして既存の処理に進める
		log.Printf("[Validation] Valid answer detected, resetting invalid count for session: %s\n", sessionID)
		if err := s.sessionValidationRepo.ResetInvalidCount(sessionID); err != nil {
			log.Printf("Warning: failed to reset invalid count: %v\n", err)
		}
		return false, "", nil
	}

	// 無効な回答と判断 -> カウントをインクリメント
	log.Printf("[Validation] Invalid answer detected for message: %s\n", userMessage)
	validation, err := s.sessionValidationRepo.IncrementInvalidCount(sessionID)
	if err != nil {
		return true, "", fmt.Errorf("failed to increment invalid count: %w", err)
	}
	log.Printf("[Validation] Invalid count incremented to: %d/3\n", validation.InvalidAnswerCount)

	var assistantText string
	if validation.InvalidAnswerCount >= 3 {
		// 3回目の無効回答 -> セッションを強制終了
		if err := s.sessionValidationRepo.TerminateSession(sessionID); err != nil {
			log.Printf("Warning: failed to terminate session: %v\n", err)
		}
		assistantText = "申し訳ございませんが、質問と関係のない内容が3回続いたため、チャットを終了させていただきます。新しいセッションで最初からやり直してください。"
	} else {
		// 1-2回目の無効回答 -> 警告メッセージ
		assistantText = fmt.Sprintf("書かれた内容にはお答えできません。質問に回答してください。（%d/3回目の警告）", validation.InvalidAnswerCount)
	}

	assistantMsg := &models.ChatMessage{
		SessionID: sessionID,
		UserID:    userID,
		Role:      "assistant",
		Content:   assistantText,
	}
	if err := s.chatMessageRepo.Create(assistantMsg); err != nil {
		return true, "", fmt.Errorf("failed to save assistant message for invalid answer: %w", err)
	}
	return true, assistantText, nil
}

// validateAnswerRelevance: 回答が質問に沿っているかを判定（文章系はキーワードベースで柔軟に判定）
func (s *ChatService) validateAnswerRelevance(ctx context.Context, question, answer string) (bool, error) {
	// 文章系の質問かどうかを判定
	isTextQuestion := isTextBasedQuestion(question)

	if isTextQuestion {
		// 文章系の質問: キーワードベースで柔軟に判定
		log.Printf("[Validation] Text-based question detected, using keyword-based validation\n")
		return isLikelyAnswer(answer, question), nil
	}

	// 選択肢型の質問: AI判定を使用
	log.Printf("[Validation] Choice-based question detected, using AI validation\n")

	systemPrompt := prompts.AnswerValidationSystemPrompt
	userPrompt := prompts.BuildAnswerValidationUserPrompt(question, answer)

	// temperature=0で安定した判定を行う
	response, err := s.aiClient.ResponsesWithTemperature(ctx, systemPrompt, userPrompt, 0.0)
	if err != nil {
		return false, fmt.Errorf("AI validation error: %w", err)
	}

	// コードフェンスを除去してJSON抽出
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// JSON構造体で検証
	type ValidationResult struct {
		Valid bool `json:"valid"`
	}

	var result ValidationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// JSONパースに失敗した場合は無効とみなす
		log.Printf("Warning: Failed to parse AI validation response: %v, response: %s\n", err, response)
		return false, nil
	}

	return result.Valid, nil
}

// isQuestion: アシスタントのメッセージが「質問」であるか粗く判定する
func isQuestion(text string) bool {
	txt := strings.TrimSpace(text)
	if txt == "" {
		return false
	}
	// 疑問符があれば質問とみなす
	if strings.ContainsAny(txt, "？?") {
		return true
	}
	// 日本語の疑問語・依頼表現が含まれるか確認
	questionWords := []string{
		"どのよう", "どの", "どう", "なぜ", "なに", "何", "いつ", "どれ", "どこ", "どなた", "どんな",
		"〜ますか", "ますか", "でしょうか",
		"教えてください", "教えて下さい", "聞かせてください", "話してください",
	}
	for _, w := range questionWords {
		if strings.Contains(txt, w) {
			return true
		}
	}
	return false
}

func normalizeQuestionText(text string) string {
	questionText := strings.TrimSpace(text)
	if questionText == "" {
		return ""
	}
	// 💡マークなどのヒント部分を除去
	if idx := strings.Index(questionText, "\n\n💡"); idx > 0 {
		questionText = questionText[:idx]
	}
	// 段落末尾の質問文を優先して抽出（前置きが付くケースを考慮）
	parts := strings.Split(questionText, "\n\n")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if isQuestion(part) {
			return part
		}
	}
	if isQuestion(questionText) {
		return questionText
	}
	return ""
}

// isTextBasedQuestion: 質問が文章系（具体的なエピソードを求める）かどうかを判定
func isTextBasedQuestion(question string) bool {
	// 選択肢型の質問のパターン
	choicePatterns := []string{
		"A)", "B)", "C)", "D)", "E)",
		"A：", "B：", "C：", "D：", "E：",
		"A、", "B、", "C、", "D、", "E、",
		"1)", "2)", "3)", "4)", "5)",
		"①", "②", "③", "④", "⑤",
		"1〜5", "1～5", "1-5",
		// numbered dot formats (1. , 1．) を選択肢として扱う
		"1.", "2.", "3.", "4.", "5.", "1．", "2．", "3．", "4．", "5．",
	}

	for _, pattern := range choicePatterns {
		if strings.Contains(question, pattern) {
			return false // 選択肢型
		}
	}

	// 文章系の質問のキーワード
	textPatterns := []string{
		"具体的", "エピソード", "経験", "体験",
		"教えてください", "教えて下さい",
		"について話して", "について教えて",
		"どのように", "どんな",
	}

	for _, pattern := range textPatterns {
		if strings.Contains(question, pattern) {
			return true // 文章系
		}
	}

	// デフォルトは文章系として扱う（柔軟に判定）
	return true
}

// isLikelyAnswer: ユーザーの入力が質問に対する「回答らしい」かを判定する簡易ロジック（フォールバック用）
// AI判定が失敗した場合の適度に柔軟なフォールバック
func isLikelyAnswer(answer, question string) bool {
	a := strings.TrimSpace(answer)

	// 十分に長い回答（15文字超）は内容があるとみなし有効
	if len([]rune(a)) > 15 {
		log.Printf("[Validation] Fallback: Long answer accepted as valid (%d chars)\n", len([]rune(a)))
		return true
	}

	// 職種選択系の質問は短い回答（単語）を許容
	isJobSelection := isJobSelectionQuestionText(question)

	// 記号のみの回答（A, B, 1, 2など）は有効
	if len([]rune(a)) <= 3 && strings.ContainsAny(a, "ABCDEabcde12345①②③④⑤") {
		log.Printf("[Validation] Fallback: Valid choice symbol: %s\n", a)
		return true
	}

	answerLower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(a, " ", ""), "　", ""))

	// 無回答パターンを先に判定し、shortValidAnswers の部分一致より優先させる。
	// 「わからない」「わからないです」等の単体のみ無効（他の文章が続く場合は有効）
	answerLowerStripped := strings.TrimRight(answerLower, "。、！？…,.!?・")
	noAnswerPatterns := []string{
		"わからない", "分からない", "わかりません", "分かりません",
		"わからないです", "分からないです",
	}
	for _, pattern := range noAnswerPatterns {
		if answerLowerStripped == pattern {
			log.Printf("[Validation] Fallback: No-answer pattern detected: %s\n", a)
			return false
		}
	}

	// 「はい」「いいえ」「うん」などの短い回答は文字数チェックより先に判定する。
	// noAnswerPatterns の後に置き、完全一致のみとすることで "ない" → "わからない" の誤マッチを防ぐ。
	// 「はい」「いいえ」「うん」などの短い回答は文字数チェックより先に判定する
	// （新卒ユーザーの短文回答を正しく有効扱いするため）
	shortValidAnswers := []string{
		"はい", "いいえ", "yes", "no", "好き", "嫌い", "得意", "苦手",
		"できる", "できない", "ある", "ない", "する", "しない",
		"うん", "そう", "ええ", "まあ", "そうです", "そうですね",
		"あります", "ないです", "あった", "なかった",
	}
	for _, valid := range shortValidAnswers {
		if answerLowerStripped == valid {
			log.Printf("[Validation] Fallback: Valid short answer: %s\n", a)
			return true
		}
	}

	// 2文字未満は無効（選択肢記号・短答キーワードは上で判定済み）
	if len([]rune(a)) < 2 {
		log.Printf("[Validation] Fallback: Too short (< 2 chars): %s\n", a)
		return false
	}

	// 挨拶・感謝などの雑談パターンは無効
	if containsGreeting(a) {
		log.Printf("[Validation] Fallback: Contains greeting: %s\n", a)
		return false
	}

	if !isJobSelection && looksLikeKeywordList(a) {
		log.Printf("[Validation] Fallback: Keyword-only list detected: %s\n", a)
		return false
	}

	// IT職種関連のキーワードを含むかチェック
	itKeywords := []string{
		"エンジニア", "プログラマ", "開発", "インフラ", "セキュリティ",
		"データ", "サイエンティスト", "アプリ", "Web", "モバイル",
		"フロントエンド", "バックエンド", "フルスタック", "DevOps",
		"クラウド", "ネットワーク", "システム", "プロジェクト",
		"技術", "スキル", "経験", "プログラミング", "コード",
	}

	hasITKeyword := false
	for _, keyword := range itKeywords {
		if strings.Contains(a, keyword) {
			hasITKeyword = true
			break
		}
	}

	// 質問文に選択肢や具体例が含まれている場合、回答側に数字や選択肢文字があれば回答とみなす
	if strings.Contains(question, "A)") || strings.Contains(question, "A：") || strings.Contains(question, "A、") {
		if strings.ContainsAny(a, "ABCDabcd1-5①②③④") {
			log.Printf("[Validation] Fallback: Contains choice character: %s\n", a)
			return true
		}
	}

	// IT関連キーワードを含む、または5文字以上なら有効（緩和）
	if hasITKeyword || len([]rune(a)) >= 5 {
		log.Printf("[Validation] Fallback: Valid answer (IT keyword or >= 5 chars): %s\n", a)
		return true
	}

	// 質問文から抽出したキーワードと回答に共通語があるかを確認する（簡易）
	qk := extractKeywords(question)
	ak := extractKeywords(a)
	common := 0
	for w := range qk {
		if ak[w] {
			common++
		}
	}

	// 共通キーワードが1つ以上あれば回答とみなす（緩和）
	if common >= 1 {
		log.Printf("[Validation] Fallback: Common keywords >= 1: %s\n", a)
		return true
	}

	// デフォルトは無効（厳格に判断）
	log.Printf("[Validation] Fallback: Default INVALID for: %s\n", a)
	return false
}

func looksLikeKeywordList(answer string) bool {
	normalized := strings.TrimSpace(answer)
	if normalized == "" {
		return false
	}
	if containsSentenceHint(normalized) {
		return false
	}

	tokens := strings.FieldsFunc(normalized, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '、', ',', '・', '/', '／', '|':
			return true
		default:
			return false
		}
	})

	return len(tokens) >= 2
}

func containsSentenceHint(s string) bool {
	hints := []string{
		"です", "ます", "ました", "した", "して", "してい", "してる",
		"いる", "ある", "なる", "たい", "たく", "と思う", "と考え", "と感じ",
		"なりたい", "したい", "つもり", "予定",
		"ので", "ため", "から", "として", "について",
	}
	for _, hint := range hints {
		if strings.Contains(s, hint) {
			return true
		}
	}
	return strings.ContainsAny(s, "。？！?!")
}

func isJobSelectionQuestionText(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	keywords := []string{
		"職種", "どの職種", "IT職種", "興味がありますか", "選んでください",
		"まだ決めていない", "番号で答えても",
	}
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// containsGreeting: 短い回答が挨拶・了承のみで構成されているかを判定する。
// 長い回答（15文字超）は本文があるとみなし false を返す。
func containsGreeting(s string) bool {
	trimmed := strings.TrimSpace(s)
	// 長い回答は挨拶のみとはみなさない
	if len([]rune(trimmed)) > 15 {
		return false
	}
	l := strings.ToLower(trimmed)
	// 完全一致で判定するパターン（誤検知を防ぐため部分一致不使用）
	exactGreetings := []string{
		"こんにちは", "こんばんは", "おはよう", "ありがとう", "ありがとうございます",
		"了解", "わかった", "わかりました", "よろしく", "ありがとうござい",
		"ok", "オッケー",
	}
	for _, g := range exactGreetings {
		if l == strings.ToLower(g) {
			return true
		}
	}
	return false
}
