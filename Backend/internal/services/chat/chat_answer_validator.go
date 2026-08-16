package chat

import (
	"Backend/internal/models"
	"Backend/internal/services/prompts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"unicode"
)

// isValidationFeedbackMessage: 無効回答の警告・強制終了メッセージか。
// これらを「直近の質問」と誤認すると、再回答が職種選択扱いなどになり連鎖で無効になる。
func isValidationFeedbackMessage(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "書かれた内容にはお答えできません") ||
		strings.Contains(trimmed, "質問と関係のない内容が3回続いた")
}

// findLastAssistantQuestion: 警告・終了メッセージを飛ばし、履歴上の直近の質問文を返す。
// 見つからない場合は空文字（呼び出し側で初回職種選択フォールバック）。
func findLastAssistantQuestion(history []models.ChatMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != "assistant" {
			continue
		}
		content := history[i].Content
		if isValidationFeedbackMessage(content) {
			continue
		}
		if isQuestion(content) {
			return content
		}
	}
	return ""
}

// checkAnswerValidity: 直近の assistant メッセージが質問かを判定し、ユーザー入力がその質問に対する有効な回答かを判定する。
// 無効な場合はアシスタントの「書かれた内容にはお答えできません」メッセージを保存して true を返す。
// 3回連続で無効な場合はセッションを強制終了する。
// 戻り値: handled(bool) - true の場合は処理を終了してよい、response(string) - 保存したアシスタント応答（ある場合）、error
func (s *ChatService) checkAnswerValidity(ctx context.Context, history []models.ChatMessage, userMessage string, userID uint, sessionID string) (bool, string, error) {
	// 警告メッセージを飛ばして実質の質問を特定する（1回ミス後の再回答が別質問扱いになるのを防ぐ）
	questionText := findLastAssistantQuestion(history)
	if questionText == "" {
		// 履歴に質問が無い初回などは職種選択を期待
		questionText = "どのようなIT職種に興味がありますか？"
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
		assistantText = "申し訳ございませんが、質問と関係のない内容が3回続いたため、チャットを終了させていただきます。「最初からやり直す」から新しいセッションを開始するか、この質問をスキップして別の話題からお試しください。"
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
		// 文章系の質問: 可能であれば埋め込みベースの意味的類似度で判定
		if s.aiClient != nil {
			qText := normalizeQuestionText(question)
			if qText == "" {
				qText = question
			}
			qEmb, err := s.aiClient.Embedding(ctx, qText)
			if err != nil {
				log.Printf("[Validation] Embedding error for question: %v\n", err)
				return isLikelyAnswer(answer, question), nil
			}
			aEmb, err := s.aiClient.Embedding(ctx, answer)
			if err != nil {
				log.Printf("[Validation] Embedding error for answer: %v\n", err)
				return isLikelyAnswer(answer, question), nil
			}

			// cosine similarity
			dot := 0.0
			qNorm := 0.0
			aNorm := 0.0
			for i := range qEmb {
				vq := float64(qEmb[i])
				va := float64(aEmb[i])
				dot += vq * va
				qNorm += vq * vq
				aNorm += va * va
			}
			if qNorm == 0 || aNorm == 0 {
				return isLikelyAnswer(answer, question), nil
			}
			cos := dot / (math.Sqrt(qNorm) * math.Sqrt(aNorm))
			log.Printf("[Validation] Embedding cosine similarity=%f\n", cos)

			// 閾値は運用で調整。0.30 を採用（緩めに設定して誤検知を抑える）
			if cos >= 0.30 {
				return true, nil
			}
			// 埋め込み不一致でも実質的な回答なら受理（誤字修正の再送・観点ズレを過剰に落とさない）
			if isLikelyAnswer(answer, question) {
				log.Printf("[Validation] Embedding below threshold but fallback accepted\n")
				return true, nil
			}
			return false, nil
		}

		// AIクライアントが無い場合は既存のキーワードベースフォールバック
		log.Printf("[Validation] Text-based question detected, using keyword-based validation\n")
		return isLikelyAnswer(answer, question), nil
	}

	// 選択肢型の質問: 記号・ラベル一致、またはその他相当の自由記述はローカルで受理
	resolved := ResolveChoiceAnswer(question, answer)
	if resolved.IsChoice {
		log.Printf("[Validation] Choice answer matched locally: %s\n", resolved.Letter)
		return true, nil
	}
	if len([]rune(strings.TrimSpace(answer))) >= 2 {
		log.Printf("[Validation] Accepting free-text answer on choice question (その他経路)\n")
		return true, nil
	}

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
	if a == "" {
		return false
	}

	// 先に明らかに無意味なパターンを弾く
	// 連続同一文字 (例: "あああああ"), ほぼ記号のみ, ランダム文字列の疑い
	if hasLongRepeatedRune(a, 5) {
		log.Printf("[Validation] Fallback: Repeated rune detected: %s\n", a)
		return false
	}
	if isMostlyPunctuation(a) {
		log.Printf("[Validation] Fallback: Mostly punctuation detected: %s\n", a)
		return false
	}
	// 日本語文字比率が極端に低く、かつ長さが短い/意味のなさそうな場合は無効
	jr := japaneseCharRatio(a)
	if jr < 0.25 && len([]rune(a)) >= 6 && !containsITKeyword(a) {
		log.Printf("[Validation] Fallback: Low japanese ratio (%.2f) and short/random: %s\n", jr, a)
		return false
	}

	// 十分に長い回答（15文字超）は基本的に内容があるとみなすが、上の無意味判定は優先
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

	// 無回答パターンを先に判定
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

	// 短いが意味ある応答（はい/いいえ等）は有効
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

	// IT関連キーワードを含む、または5文字以上なら有効
	if containsITKeyword(a) || len([]rune(a)) >= 5 {
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

// hasLongRepeatedRune returns true if any rune repeats consecutively >= n times
func hasLongRepeatedRune(s string, n int) bool {
	if n <= 1 {
		return false
	}
	count := 1
	var prev rune
	for i, r := range s {
		if i == 0 {
			prev = r
			continue
		}
		if r == prev {
			count++
			if count >= n {
				return true
			}
		} else {
			count = 1
			prev = r
		}
	}
	return false
}

// japaneseCharRatio returns ratio of runes that are Hiragana/Katakana/CJK
func japaneseCharRatio(s string) float64 {
	if s == "" {
		return 0
	}
	total := 0
	j := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if (r >= '\u3040' && r <= '\u30ff') || (r >= '\u4e00' && r <= '\u9faf') || (r >= '\u3400' && r <= '\u4dbf') {
			j++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(j) / float64(total)
}

// isMostlyPunctuation returns true if over 80% of runes are punctuation/symbols
func isMostlyPunctuation(s string) bool {
	total := 0
	p := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if unicode.IsPunct(r) || unicode.IsSymbol(r) || strings.ContainsRune("。、！？….,!?・", r) {
			p++
		}
	}
	if total == 0 {
		return false
	}
	return float64(p)/float64(total) >= 0.8
}

// containsITKeyword checks for presence of IT-related keywords
func containsITKeyword(s string) bool {
	lower := strings.ToLower(s)
	itKeywords := []string{
		"エンジニア", "プログラマ", "開発", "インフラ", "セキュリティ",
		"データ", "サイエンティスト", "アプリ", "Web", "モバイル",
		"フロントエンド", "バックエンド", "フルスタック", "DevOps",
		"クラウド", "ネットワーク", "システム", "プロジェクト",
		"技術", "スキル", "経験", "プログラミング", "コード",
		"backend", "frontend", "react", "python", "java", "javascript",
		"typescript", "go", "golang", "aws", "gcp", "azure", "docker",
		"kubernetes", "sql", "mysql", "postgres", "api", "devops",
	}
	for _, k := range itKeywords {
		if strings.Contains(s, k) || strings.Contains(lower, strings.ToLower(k)) {
			return true
		}
	}
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
	// 職種・興味の明示。これだけで職種選択とみなしてよい。
	primary := []string{
		"職種", "IT職種", "どの職種",
		"興味がありますか", "興味の方向", "どれに興味",
		"まだ決めていない", "番号で答えても",
		"好きな作業", "どんな作業", "作業が好き",
	}
	for _, keyword := range primary {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	// 「選んでください」「近いですか」は通常の面接MCQにも出るため、
	// 職種・役割の文脈があるときだけ職種選択とみなす。
	selectionCue := strings.Contains(text, "選んでください") ||
		strings.Contains(text, "どれが近い") ||
		strings.Contains(text, "近いですか") ||
		strings.Contains(text, "一番近い")
	if !selectionCue {
		return false
	}
	return containsJobSelectionContext(text)
}

// containsJobSelectionContext: 職種clarification系の選択質問かどうかを文脈語で判定する。
// 「開発」「データ」など経験談MCQにも出る語は入れない（誤って職種判定に再突入するのを防ぐ）。
func containsJobSelectionContext(text string) bool {
	cues := []string{
		"職種", "興味", "まだ決めていない",
		"エンジニア", "営業", "マーケ", "人事", "デザイナー",
		"作業が好き", "好きな作業", "どんな作業",
		"どの分野", "どの領域", "キャリアの方向", "仕事の向き",
		"役割", "ポジション",
	}
	for _, cue := range cues {
		if strings.Contains(text, cue) {
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
