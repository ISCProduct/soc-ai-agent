package services

import (
	"Backend/internal/models"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// AnswerEvaluator 回答の評価サービス
type AnswerEvaluator struct{}

func NewAnswerEvaluator() *AnswerEvaluator {
	return &AnswerEvaluator{}
}

// EvaluationResult 評価結果
type EvaluationResult struct {
	Score           int      `json:"score"`
	Confidence      string   `json:"confidence"` // "high", "medium", "low"
	MatchedKeywords []string `json:"matched_keywords"`
	AppliedRules    []string `json:"applied_rules"`
	NeedsFollowUp   bool     `json:"needs_follow_up"`
	FollowUpTrigger string   `json:"follow_up_trigger"`
	Explanation     string   `json:"explanation"`
}

// Evaluate ルールベースで回答を評価
func (e *AnswerEvaluator) Evaluate(question *models.PredefinedQuestion, answer string) (*EvaluationResult, error) {
	result := &EvaluationResult{
		Score:           0,
		MatchedKeywords: []string{},
		AppliedRules:    []string{},
	}

	answerLower := strings.ToLower(answer)
	answerLength := len([]rune(answer))

	// 1. 回答の長さチェック（基本的な信頼性判定）
	if answerLength < 10 {
		result.Score -= 3
		result.Confidence = "low"
		result.NeedsFollowUp = true
		result.FollowUpTrigger = "too_short"
		result.Explanation = "回答が短すぎます（10文字未満）"
		return result, nil
	}

	// 2. ネガティブキーワードチェック
	var negativeKeywords []string
	if question.NegativeKeywords != "" {
		json.Unmarshal([]byte(question.NegativeKeywords), &negativeKeywords)
	}

	for _, keyword := range negativeKeywords {
		if strings.Contains(answerLower, strings.ToLower(keyword)) {
			result.Score -= 2
			result.MatchedKeywords = append(result.MatchedKeywords, fmt.Sprintf("-%s", keyword))
			result.NeedsFollowUp = true
			result.FollowUpTrigger = "negative_keyword"
		}
	}

	// 3. ポジティブキーワードチェック
	var positiveKeywords []string
	if question.PositiveKeywords != "" {
		json.Unmarshal([]byte(question.PositiveKeywords), &positiveKeywords)
	}

	matchedCount := 0
	for _, keyword := range positiveKeywords {
		if strings.Contains(answerLower, strings.ToLower(keyword)) {
			result.Score += 1
			result.MatchedKeywords = append(result.MatchedKeywords, fmt.Sprintf("+%s", keyword))
			matchedCount++
		}
	}

	// 4. スコアリングルールの適用
	var scoreRules []models.ScoreRule
	if question.ScoreRules != "" {
		json.Unmarshal([]byte(question.ScoreRules), &scoreRules)
	}

	for _, rule := range scoreRules {
		if e.evaluateRule(rule, answer, answerLength) {
			result.Score += rule.ScoreChange
			result.AppliedRules = append(result.AppliedRules, rule.Description)
		}
	}

	// 5. 信頼度の判定
	if matchedCount == 0 && result.Score <= 0 {
		result.Confidence = "low"
		result.NeedsFollowUp = true
		result.FollowUpTrigger = "no_keywords"
		result.Explanation = "関連キーワードが見つかりませんでした"
	} else if answerLength > 100 && matchedCount >= 2 {
		result.Confidence = "high"
		result.Explanation = "具体的で詳細な回答です"
	} else {
		result.Confidence = "medium"
		result.Explanation = "ある程度の評価ができました"
	}

	// 6. 追加質問が必要かチェック
	var followUpRules []models.FollowUpRule
	if question.FollowUpRules != "" {
		json.Unmarshal([]byte(question.FollowUpRules), &followUpRules)
	}

	for _, rule := range followUpRules {
		if e.shouldTriggerFollowUp(rule, result) {
			result.NeedsFollowUp = true
			result.FollowUpTrigger = rule.Trigger
			break
		}
	}

	return result, nil
}

// evaluateRule 個別のスコアリングルールを評価
func (e *AnswerEvaluator) evaluateRule(rule models.ScoreRule, answer string, answerLength int) bool {
	answerLower := strings.ToLower(answer)

	switch rule.Condition {
	case "contains_any":
		// いずれかのキーワードを含む
		for _, keyword := range rule.Keywords {
			if strings.Contains(answerLower, strings.ToLower(keyword)) {
				return true
			}
		}
		return false

	case "contains_all":
		// すべてのキーワードを含む
		for _, keyword := range rule.Keywords {
			if !strings.Contains(answerLower, strings.ToLower(keyword)) {
				return false
			}
		}
		return true

	case "length_gt":
		// 文字数が指定値より大きい
		if len(rule.Keywords) > 0 {
			threshold := 0
			fmt.Sscanf(rule.Keywords[0], "%d", &threshold)
			return answerLength > threshold
		}
		return false

	case "length_lt":
		// 文字数が指定値より小さい
		if len(rule.Keywords) > 0 {
			threshold := 0
			fmt.Sscanf(rule.Keywords[0], "%d", &threshold)
			return answerLength < threshold
		}
		return false

	case "regex":
		// 正規表現マッチ
		if len(rule.Keywords) > 0 {
			pattern := rule.Keywords[0]
			matched, err := regexp.MatchString(pattern, answer)
			if err != nil {
				return false
			}
			return matched
		}
		return false

	case "has_example":
		// 具体例を含んでいるか（「例えば」「たとえば」「〜した時」など）
		examplePatterns := []string{
			"例えば", "たとえば", "具体的には", "実際に", "した時", "したとき",
			"経験", "〜で", "〜では", "ことがあ",
		}
		for _, pattern := range examplePatterns {
			if strings.Contains(answerLower, pattern) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// shouldTriggerFollowUp 追加質問が必要か判定
func (e *AnswerEvaluator) shouldTriggerFollowUp(rule models.FollowUpRule, result *EvaluationResult) bool {
	switch rule.Trigger {
	case "low_confidence":
		return result.Confidence == "low"

	case "high_score":
		return result.Score >= 5

	case "no_keywords":
		return len(result.MatchedKeywords) == 0

	case "negative_keyword":
		return result.FollowUpTrigger == "negative_keyword"

	default:
		return false
	}
}

// GetConfidenceLevel スコアから信頼度レベルを取得
func (e *AnswerEvaluator) GetConfidenceLevel(score int, keywordCount int, answerLength int) string {
	if score <= 0 || keywordCount == 0 {
		return "low"
	}

	if score >= 5 && keywordCount >= 2 && answerLength > 50 {
		return "high"
	}

	return "medium"
}

type PrecheckAction string

const (
	PrecheckIgnore  PrecheckAction = "ignore"
	PrecheckSkip    PrecheckAction = "skip"
	PrecheckNoScore PrecheckAction = "no_score"
	PrecheckScore   PrecheckAction = "score"
)

type HumanScoreResult struct {
	Action          PrecheckAction
	Score           int
	CategoryID      string
	RubricID        string
	DimensionScores map[string]int
	Penalties       []string
	Boosts          []string
	Reason          string
}

type questionMeta struct {
	QuestionType     string
	ChoiceSemantics  string
	ChoiceOptionText map[string]string
}

func (e *AnswerEvaluator) EvaluateHumanScoring(question, answer string, isChoice bool, jobRoleSet bool, meta *questionMeta) HumanScoreResult {
	precheck := e.precheckHuman(answer, isChoice, jobRoleSet)
	if precheck.Action != PrecheckScore {
		return precheck
	}

	if isChoice {
		score := e.scoreChoice(answer, meta)
		return HumanScoreResult{
			Action:     PrecheckScore,
			Score:      score,
			CategoryID: "generic",
			RubricID:   "choice_default",
		}
	}

	category := e.categorizeQuestion(question)
	rubric := rubricForCategory(category)
	signals := extractSignals(answer, category)
	dimensionScores := scoreDimensions(rubric, signals, answer)
	rawScore := scoreFromDimensions(rubric, dimensionScores)
	score, penalties, boosts := applyPenaltiesAndBoosts(rawScore, signals)

	return HumanScoreResult{
		Action:          PrecheckScore,
		Score:           score,
		CategoryID:      category,
		RubricID:        rubric,
		DimensionScores: dimensionScores,
		Penalties:       penalties,
		Boosts:          boosts,
	}
}

func (e *AnswerEvaluator) precheckHuman(answer string, isChoice bool, jobRoleSet bool) HumanScoreResult {
	answerTrimmed := strings.TrimSpace(answer)
	_ = jobRoleSet

	if isChoice {
		return HumanScoreResult{Action: PrecheckScore}
	}

	normalizedAnswer := strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(answerTrimmed), " ", ""), "　", "")
	// 末尾の句読点・記号を除去した形でも比較できるよう正規化する
	normalizedStripped := strings.TrimRight(normalizedAnswer, "。、！？…,.!?・")

	// skipPhrases を先に判定し、スキップが短答リストより優先されるようにする。
	// 末尾記号を除去した上で完全一致（"なし！" 等も捕捉）
	skipPhrases := []string{
		"わからない", "分からない", "わかりません", "分かりません", "特にない", "特になし", "なし",
	}
	for _, phrase := range skipPhrases {
		if normalizedStripped == phrase {
			return HumanScoreResult{Action: PrecheckSkip, Reason: "skip_phrase"}
		}
	}

	// 新卒ユーザーの短文回答（「はい」「ある」等）を評価対象として通すため、
	// shortValidAnswers に該当する場合は文字数に関わらず通常のスコアリングへ進める。
	// skipPhrases の後に置き、完全一致のみとすることで "ない" → "わからない" の誤マッチを防ぐ。
	shortValidAnswers := []string{
		"はい", "いいえ", "yes", "no", "好き", "嫌い", "得意", "苦手",
		"できる", "できない", "ある", "ない", "する", "しない",
		"うん", "そう", "ええ", "まあ", "そうです", "そうですね",
		"あります", "ないです", "あった", "なかった",
	}
	for _, valid := range shortValidAnswers {
		if normalizedStripped == valid {
			return HumanScoreResult{Action: PrecheckScore}
		}
	}

	// 完全に空または1文字以下は無視
	if len([]rune(answerTrimmed)) < 2 {
		return HumanScoreResult{Action: PrecheckIgnore, Reason: "too_short_ignore"}
	}

	// skipPhrases（上記と同一スライスを再利用）を非ストリップ形式でも照合
	// 句読点付き「なし。」等も捕捉するため trailing 記号を含む形でも確認
	for _, phrase := range skipPhrases {
		if normalizedAnswer == phrase || normalizedAnswer == phrase+"。" || normalizedAnswer == phrase+"、" {
			return HumanScoreResult{Action: PrecheckSkip, Reason: "skip_phrase"}
		}
	}

	// 5文字未満の短文は最小スコアを付与（PrecheckNoScoreから変更）
	if len([]rune(answerTrimmed)) < 5 {
		return HumanScoreResult{Action: PrecheckScore, Reason: "short_but_valid"}
	}

	// 10文字未満も評価対象とする（新卒の短答に対応）
	return HumanScoreResult{Action: PrecheckScore}
}

// InferCategory は質問文からカテゴリを推論する（テスト用に公開）
func (e *AnswerEvaluator) InferCategory(question string) string {
	return e.categorizeQuestion(question)
}

func (e *AnswerEvaluator) categorizeQuestion(question string) string {
	q := strings.ToLower(question)
	if containsAny(q, []string{"興味", "きっかけ", "理由", "魅力", "なぜ"}) {
		return "motivation"
	}
	if containsAny(q, []string{"作った", "開発", "実装", "制作", "プロジェクト", "成果物", "github"}) {
		return "experience"
	}
	if containsAny(q, []string{"意見", "食い違い", "合意", "調整", "衝突", "まとめ", "折衷", "合意形成"}) {
		return "collaboration"
	}
	if containsAny(q, []string{"itに詳しくない", "職員", "説明", "理解確認", "伝え方", "使い方", "現場"}) {
		return "communication_non_it"
	}
	if containsAny(q, []string{"ui", "ux", "使いやすさ", "試して", "ユーザ", "改善", "反応", "導線"}) {
		return "ui_ux"
	}
	return "generic"
}

func rubricForCategory(category string) string {
	switch category {
	case "collaboration":
		return "collaboration_rubric"
	case "communication_non_it":
		return "communication_non_it_rubric"
	case "ui_ux":
		return "ui_ux_rubric"
	default:
		return "generic_text_rubric"
	}
}

type signalSet struct {
	hasConcreteExample   bool
	hasAction            bool
	hasResult            bool
	hasReason            bool
	hasNumbersOrTime     bool
	hasCollaborationTerm bool
	hasNonITTerm         bool
	hasUxTerm            bool
	contradiction        bool
}

func extractSignals(answer string, category string) signalSet {
	lower := strings.ToLower(answer)
	return signalSet{
		hasConcreteExample: containsAny(lower, []string{"例えば", "たとえば", "具体的", "実際に", "経験", "した時", "したとき"}),
		hasAction:          containsAny(lower, []string{"取り組", "実施", "作成", "作った", "実装", "改善", "対応", "開発", "設計", "検証"}),
		hasResult:          containsAny(lower, []string{"結果", "成果", "達成", "改善された", "向上", "成功", "失敗"}),
		hasReason:          containsAny(lower, []string{"理由", "なぜ", "ので", "ため", "から", "だから"}),
		hasNumbersOrTime:   regexp.MustCompile(`[0-9]`).MatchString(lower) || containsAny(lower, []string{"ヶ月", "年", "週間", "日間", "%", "人", "回"}),
		hasCollaborationTerm: containsAny(lower, []string{
			"合意", "調整", "衝突", "折衷", "意見", "まとめ",
		}),
		hasNonITTerm: containsAny(lower, []string{
			"itに詳しくない", "非エンジニア", "職員", "現場", "利用者",
		}),
		hasUxTerm: containsAny(lower, []string{
			"ui", "ux", "ユーザ", "導線", "使いやす", "反応", "テスト",
		}),
		contradiction: false,
	}
}

func scoreDimensions(rubric string, signals signalSet, answer string) map[string]int {
	length := len([]rune(strings.TrimSpace(answer)))
	scores := map[string]int{
		"relevance":   1,
		"specificity": 0,
		"reasoning":   0,
		"credibility": 1,
	}

	// 理想的・定型的な回答に対するペナルティ
	isTooPerfect := !signals.hasReason && !signals.hasNumbersOrTime && length > 50 && signals.hasAction && signals.hasResult

	if rubric == "collaboration_rubric" && !signals.hasCollaborationTerm {
		scores["relevance"] = 1
	} else if rubric == "communication_non_it_rubric" && !signals.hasNonITTerm {
		scores["relevance"] = 1
	} else if rubric == "ui_ux_rubric" && !signals.hasUxTerm {
		scores["relevance"] = 1
	}

	if signals.hasConcreteExample && signals.hasAction {
		scores["relevance"] = 3
	} else if length >= 20 {
		scores["relevance"] = 2
	}

	if signals.hasConcreteExample && signals.hasNumbersOrTime {
		scores["specificity"] = 3
	} else if signals.hasConcreteExample {
		scores["specificity"] = 2
	} else if length >= 20 {
		scores["specificity"] = 1
	}

	if signals.hasReason && (signals.hasAction || signals.hasResult) {
		scores["reasoning"] = 3
	} else if signals.hasReason {
		scores["reasoning"] = 2
	} else if length >= 30 {
		scores["reasoning"] = 1
	}

	if signals.contradiction {
		scores["credibility"] = 0 // 矛盾あり: 最低評価
	} else if isTooPerfect {
		scores["credibility"] = 1 // 理由・数値のない定型回答: 信頼度を落とす
	} else if signals.hasNumbersOrTime {
		scores["credibility"] = 3
	} else if signals.hasConcreteExample {
		scores["credibility"] = 2
	}

	return scores
}

func scoreFromDimensions(rubric string, dimensionScores map[string]int) int {
	weights := map[string]float64{
		"relevance":   0.35,
		"specificity": 0.30,
		"reasoning":   0.20,
		"credibility": 0.15,
	}

	switch rubric {
	case "collaboration_rubric":
		weights["specificity"] = 0.35
		weights["reasoning"] = 0.30
	case "communication_non_it_rubric":
		weights["relevance"] = 0.35
		weights["reasoning"] = 0.30
	case "ui_ux_rubric":
		weights["specificity"] = 0.35
	}

	sum := 0.0
	maxSum := 0.0
	for key, weight := range weights {
		sum += weight * float64(dimensionScores[key])
		maxSum += weight * 3.0
	}
	if maxSum == 0 {
		return 0
	}
	score := (sum / maxSum) * 100.0
	return int(math.Round(score))
}

func applyPenaltiesAndBoosts(score int, signals signalSet) (int, []string, []string) {
	penalties := []string{}
	boosts := []string{}
	finalScore := score

	if !signals.hasConcreteExample && !signals.hasAction {
		finalScore -= 10
		penalties = append(penalties, "too_generic")
	}
	if signals.contradiction {
		finalScore -= 20
		penalties = append(penalties, "contradiction")
	}
	if signals.hasNumbersOrTime {
		finalScore += 5
		boosts = append(boosts, "evidence")
	}

	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	return finalScore, penalties, boosts
}

func (e *AnswerEvaluator) scoreChoice(answer string, meta *questionMeta) int {
	choice := strings.ToUpper(strings.TrimSpace(answer))
	direction := "neutral"
	if meta != nil && meta.ChoiceSemantics != "" {
		direction = meta.ChoiceSemantics
	}

	switch direction {
	case "higher_is_better":
		return mapChoice(choice, []int{100, 80, 60, 40, 20})
	case "lower_is_better":
		return mapChoice(choice, []int{20, 40, 60, 80, 100})
	default:
		return mapChoice(choice, []int{100, 80, 60, 40, 20})
	}
}

func mapChoice(choice string, values []int) int {
	switch choice {
	case "A", "1":
		return values[0]
	case "B", "2":
		return values[1]
	case "C", "3":
		return values[2]
	case "D", "4":
		return values[3]
	case "E", "5":
		return values[4]
	default:
		return 60
	}
}

func containsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
