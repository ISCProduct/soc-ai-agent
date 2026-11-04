package aibench

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"

	"Backend/internal/services/prompts"
)

// chatAnswerTarget はチャット診断の回答品質を LLM に評価させる処理（answer_evaluator.go の
// llmEvaluate）を評価する。測るのは「LLM の点が人のラベルと同じ順に並ぶか」(SOCAIAGENT-450)。
//
// この点は user_weight_scores からマッチングまで伝わるが、人の判断との一致は測られていなかった。
// DB・ルールベース評価には触らない。LLM へ渡るのはプロンプト2つだけなので、
// 本番と同じ prompts.AnswerQualitySystemPrompt / BuildAnswerQualityUserPrompt を使う。
type chatAnswerTarget struct {
	model string
}

// 本番の呼び出しパラメータ（answer_evaluator.go の llmEvaluate と同じ値）。
// 違う値にすると、別物を測ることになる（TestChatAnswerTarget_Runの呼び出し内容 が固定する）。
//
// 本番で llmEvaluate が使われるのは EvaluateHumanScoringWithContext の1経路だけで、
// 選択式でなく、回答が30文字以下かルール点が20〜55のときに限られ、ルール点と
// 重み 0.35/0.55 でブレンドされる。このターゲットは**全件に LLM を適用した LLM 単体の弁別力**
// を測っており、本番の user_weight_scores への影響そのものではない。
const (
	chatAnswerTemperature = 0.2
	chatAnswerMaxTokens   = 256
	// chatAnswerPromptOverheadTokens はシステムプロンプト＋ユーザープロンプトの定型部分の概算。
	// TestChatAnswerPromptOverheadTokens が実測との乖離を検出する。
	chatAnswerPromptOverheadTokens = 526
)

func newChatAnswerTarget(modelOverride string) Target {
	// 本番の既定は openai.Client の DefaultModel（AI_TEXT_MODEL → OPENAI_MODEL → gpt-4o-mini）。
	m := strings.TrimSpace(modelOverride)
	if m == "" {
		m = strings.TrimSpace(os.Getenv("AI_TEXT_MODEL"))
	}
	if m == "" {
		m = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if m == "" {
		m = "gpt-4o-mini"
	}
	return &chatAnswerTarget{model: m}
}

func (t *chatAnswerTarget) Name() string     { return TargetChatAnswer }
func (t *chatAnswerTarget) Model() string    { return t.model }
func (t *chatAnswerTarget) Endpoint() string { return "openai:/chat/completions" }

func (t *chatAnswerTarget) EstimateTokens(c Case) (int, int) {
	return ApproxTokensJA(c.Input.Question) + ApproxTokensJA(c.Input.Answer) + chatAnswerPromptOverheadTokens, chatAnswerMaxTokens
}

func (t *chatAnswerTarget) Run(ctx context.Context, c Case) Observation {
	userPrompt := prompts.BuildAnswerQualityUserPrompt(c.Input.Question, c.Input.Answer)
	res := CallChatCompletions(ctx, t.model, prompts.AnswerQualitySystemPrompt, userPrompt, chatAnswerTemperature, chatAnswerMaxTokens, true)
	return evaluateChatAnswerResponse(newObservation(c), res)
}

// chatAnswerResponse はプロンプトが指定している出力形式で、本番の LLMAnswerEvaluation と
// 同じフィールド・型で受ける。型が違う出力（specificity が小数など）は本番の Unmarshal が
// 全体を失敗させて評価ごと捨てるので、ここでも同じ読み方で破損に数える。
// Score だけは欠落（本番は 0 点として通す）を数えるためポインタで受ける。
type chatAnswerResponse struct {
	Score        *int   `json:"score"`
	Confidence   string `json:"confidence"`
	Specificity  int    `json:"specificity"`
	Authenticity int    `json:"authenticity"`
	Consistency  int    `json:"consistency"`
	Explanation  string `json:"explanation"`
}

var chatAnswerConfidences = []string{"high", "medium", "low"}

// evaluateChatAnswerResponse は LLM の生出力を指標へ落とす（純粋関数）。
//
// **本番の読み方に合わせる**（answer_evaluator.go の llmEvaluate）。他の target と違う点が2つある。
//   - 本番は出力をそのまま json.Unmarshal する（前後の散文やコードフェンスは剥がさない）。
//     読めなければこの評価は捨てられる（ルールベースの結果が残る）ので、ここも破損に数える。
//   - score が欠けても Unmarshal は成功し、0 点として採用される。破損にすると本番より
//     平均が高く出るので、0 点として数え、score_missing の指示違反に残す。
//
// 範囲外の score は本番と同じく丸める（score_out_of_range）。不正な confidence は
// 本番が medium に置き換えて通すので破損にしない（confidence_invalid）。
func evaluateChatAnswerResponse(o Observation, res CallResult) Observation {
	o.LatencyMS = res.LatencyMS
	o.PromptTokens = res.PromptTokens
	o.CompletionTokens = res.CompletionTokens

	if res.Truncated {
		o.Broken, o.BrokenReason, o.Error = true, BrokenTruncated, "max_tokens に到達"
		return o
	}
	if res.Err != nil {
		reason := BrokenCallFailed
		if res.Unmeasured {
			reason = BrokenNetwork
		}
		o.Broken, o.BrokenReason, o.Error, o.Fatal = true, reason, res.Err.Error(), res.Fatal
		return o
	}

	var resp chatAnswerResponse
	if err := json.Unmarshal([]byte(res.Text), &resp); err != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenJSONInvalid, err.Error()
		return o
	}

	score := 0
	if resp.Score == nil {
		o.Violations = append(o.Violations, "score_missing")
	} else {
		score = *resp.Score
	}
	if score < 0 || score > 100 {
		o.Violations = append(o.Violations, "score_out_of_range")
		score = min(max(score, 0), 100)
	}
	o.RawScore = float64(score)
	o.Score = float64(score) / 100

	if !slices.Contains(chatAnswerConfidences, resp.Confidence) {
		o.Violations = append(o.Violations, "confidence_invalid")
	}
	return o
}
