package chat

import (
	"Backend/internal/models"
	internalOpenAI "Backend/internal/openai"
	"Backend/internal/services/prompts"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
)

// AnswerEvaluator 回答の評価サービス
type AnswerEvaluator struct {
	llmClient *internalOpenAI.Client // nil の場合はLLM評価を無効化
}

func NewAnswerEvaluator() *AnswerEvaluator {
	return &AnswerEvaluator{}
}

// NewAnswerEvaluatorWithLLM LLMフォールバックを有効にして生成する
func NewAnswerEvaluatorWithLLM(client *internalOpenAI.Client) *AnswerEvaluator {
	return &AnswerEvaluator{llmClient: client}
}

// LLMAnswerEvaluation はLLM評価のレスポンス構造体
type LLMAnswerEvaluation struct {
	Score        int    `json:"score"`
	Confidence   string `json:"confidence"`
	Specificity  int    `json:"specificity"`
	Authenticity int    `json:"authenticity"`
	Consistency  int    `json:"consistency"`
	Explanation  string `json:"explanation"`
}

// llmEvaluate は質問と回答をLLMで評価する。llmClientがnilの場合はnilを返す。
func (e *AnswerEvaluator) llmEvaluate(ctx context.Context, question, answer string) *LLMAnswerEvaluation {
	if e.llmClient == nil {
		return nil
	}
	userPrompt := prompts.BuildAnswerQualityUserPrompt(question, answer)
	raw, err := e.llmClient.ChatCompletionJSON(ctx, prompts.AnswerQualitySystemPrompt, userPrompt, 0.2, 256)
	if err != nil {
		log.Printf("[AnswerEvaluator] LLM評価エラー: %v", err)
		return nil
	}
	var eval LLMAnswerEvaluation
	if err := json.Unmarshal([]byte(raw), &eval); err != nil {
		log.Printf("[AnswerEvaluator] LLMレスポンスパースエラー: %v raw=%s", err, raw)
		return nil
	}
	if eval.Score < 0 {
		eval.Score = 0
	}
	if eval.Score > 100 {
		eval.Score = 100
	}
	if eval.Confidence != "high" && eval.Confidence != "medium" && eval.Confidence != "low" {
		eval.Confidence = "medium"
	}
	log.Printf("[AnswerEvaluator] LLM評価完了: score=%d confidence=%s specificity=%d authenticity=%d consistency=%d",
		eval.Score, eval.Confidence, eval.Specificity, eval.Authenticity, eval.Consistency)
	return &eval
}

// EvaluateWithLLMFallback はルールベース評価を行い、信頼度が "low" の場合のみLLMで再評価する（Phase 1）。
// llmClientがnilの場合はEvaluateと同一の動作をする。
func (e *AnswerEvaluator) EvaluateWithLLMFallback(ctx context.Context, question *models.PredefinedQuestion, questionText, answer string) (*EvaluationResult, error) {
	result, err := e.Evaluate(question, answer)
	if err != nil {
		return result, err
	}
	if result.Confidence != "low" || e.llmClient == nil {
		return result, nil
	}

	llmResult := e.llmEvaluate(ctx, questionText, answer)
	if llmResult == nil {
		return result, nil
	}

	// LLMがより高い信頼度を判定した場合は結果を上書きする
	if llmResult.Confidence == "high" || llmResult.Confidence == "medium" {
		result.Confidence = llmResult.Confidence
		result.Score = int(math.Round(float64(llmResult.Score) / 10.0))
		result.Explanation = llmResult.Explanation
		result.NeedsFollowUp = false
		result.FollowUpTrigger = ""
	}
	return result, nil
}

// EvaluateHybrid はルールベースとLLM評価を並列実行し、加重平均でスコアを合成する（Phase 2）。
// llmClientがnilの場合はEvaluateと同一の動作をする。
// ruleWeight: ルールベースの重み（0.0〜1.0）。llmWeight = 1.0 - ruleWeight。
func (e *AnswerEvaluator) EvaluateHybrid(ctx context.Context, question *models.PredefinedQuestion, questionText, answer string, ruleWeight float64) (*EvaluationResult, error) {
	if e.llmClient == nil {
		return e.Evaluate(question, answer)
	}

	var (
		ruleResult *EvaluationResult
		ruleErr    error
		llmResult  *LLMAnswerEvaluation
		wg         sync.WaitGroup
		mu         sync.Mutex
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		r, err := e.Evaluate(question, answer)
		mu.Lock()
		ruleResult, ruleErr = r, err
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		r := e.llmEvaluate(ctx, questionText, answer)
		mu.Lock()
		llmResult = r
		mu.Unlock()
	}()
	wg.Wait()

	if ruleErr != nil {
		return ruleResult, ruleErr
	}
	if llmResult == nil {
		return ruleResult, nil
	}

	// スコアを加重平均で合成（ruleResultのスコアは0-10スケール想定）
	llmWeight := 1.0 - ruleWeight
	ruleScore := float64(ruleResult.Score) * 10.0
	blended := ruleWeight*ruleScore + llmWeight*float64(llmResult.Score)
	ruleResult.Score = int(math.Round(blended / 10.0))

	// より楽観的な信頼度を採用する
	ruleResult.Confidence = MergeConfidence(ruleResult.Confidence, llmResult.Confidence)
	if ruleResult.Confidence != "low" {
		ruleResult.NeedsFollowUp = false
		ruleResult.FollowUpTrigger = ""
	}
	if llmResult.Explanation != "" {
		ruleResult.Explanation = llmResult.Explanation
	}
	return ruleResult, nil
}

// MergeConfidence は2つの信頼度のうち高い方を返す
func MergeConfidence(a, b string) string {
	rank := map[string]int{"low": 0, "medium": 1, "high": 2}
	if rank[a] >= rank[b] {
		return a
	}
	return b
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

	// 5. 信頼度の判定（文字数ではなくキーワードマッチとスコアで判断）
	if matchedCount == 0 && result.Score <= 0 {
		result.Confidence = "low"
		result.NeedsFollowUp = true
		result.FollowUpTrigger = "no_keywords"
		result.Explanation = "関連キーワードが見つかりませんでした"
	} else if matchedCount >= 2 {
		result.Confidence = "high"
		result.Explanation = "関連キーワードを複数含む回答です"
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
