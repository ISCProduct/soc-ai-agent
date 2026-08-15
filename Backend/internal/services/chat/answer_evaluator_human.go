package chat

import (
	"context"
	"math"
	"strings"
)

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
	score, penalties, boosts := applyPenaltiesAndBoosts(rawScore, signals, len([]rune(strings.TrimSpace(answer))))

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

// shouldBlendLLMForHumanScore は短文または境界帯スコア時のみ LLM 合成する (#561 / #557)。
func shouldBlendLLMForHumanScore(answer string, rule HumanScoreResult) bool {
	if rule.Action != PrecheckScore {
		return false
	}
	n := len([]rune(strings.TrimSpace(answer)))
	if n > 0 && n <= 30 {
		return true
	}
	return rule.Score >= 20 && rule.Score <= 55
}

// EvaluateHumanScoringWithContext はルール評価後、短文・境界値のみ LLM とハイブリッド合成する。
// llmClient が nil の場合は EvaluateHumanScoring と同一。
func (e *AnswerEvaluator) EvaluateHumanScoringWithContext(
	ctx context.Context,
	question, answer string,
	isChoice, jobRoleSet bool,
	meta *questionMeta,
) HumanScoreResult {
	rule := e.EvaluateHumanScoring(question, answer, isChoice, jobRoleSet, meta)
	if e.llmClient == nil || isChoice || !shouldBlendLLMForHumanScore(answer, rule) {
		return rule
	}

	llmResult := e.llmEvaluate(ctx, question, answer)
	if llmResult == nil {
		return rule
	}

	n := len([]rune(strings.TrimSpace(answer)))
	ruleWeight := 0.55
	if n > 0 && n <= 30 {
		ruleWeight = 0.35 // 短文は LLM（内容品質）比重を上げる
	}
	blended := int(math.Round(ruleWeight*float64(rule.Score) + (1-ruleWeight)*float64(llmResult.Score)))
	if blended < 0 {
		blended = 0
	}
	if blended > 100 {
		blended = 100
	}
	rule.Score = blended
	rule.Boosts = append(rule.Boosts, "llm_hybrid")
	if llmResult.Explanation != "" {
		rule.Reason = llmResult.Explanation
	}
	return rule
}

// floorEngagedShortScore は関与のある短文が Score=0 で進捗から落ちないよう下限を設ける (#561)。
func floorEngagedShortScore(answer string, result HumanScoreResult) HumanScoreResult {
	if result.Action != PrecheckScore || result.Score > 0 {
		return result
	}
	n := len([]rune(strings.TrimSpace(answer)))
	if n == 0 || n > 30 {
		return result
	}
	signals := extractSignals(answer, result.CategoryID)
	if !signals.hasEngagement() {
		return result
	}
	result.Score = 15
	result.Boosts = append(result.Boosts, "engaged_short_floor")
	return result
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
