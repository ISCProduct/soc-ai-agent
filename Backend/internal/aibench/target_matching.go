package aibench

import (
	"context"
	"os"
	"regexp"
	"strings"

	"Backend/internal/services/matching"
)

// matchingReasonTarget はマッチング理由の生成（matching_service.go の GenerateMatchReason）を
// 評価する（#1782）。
//
// 本番と同じプロンプト・温度・出力上限（/responses、max_output_tokens=100、上限到達なら
// 200 で1回だけやり直す）で呼ぶ。点はプロンプトの指示のうち機械的に確かめられる5項目を
// 満たした割合。入力に無い企業の事実を作っていないか（根拠のない記述）は判定していない。
type matchingReasonTarget struct {
	model string
}

const (
	// 本番の openai.Client.ResponsesWithTemperature が使う出力上限（初回と、上限到達時のやり直し）。
	matchingReasonMaxOutputTokens      = 100
	matchingReasonRetryMaxOutputTokens = 200
	matchingReasonPromptOverheadTokens = 700
	matchingReasonMinChars             = 120
	matchingReasonMaxChars             = 220
)

// matchingReasonFormatPattern は見出し・箇条書き・JSON（プロンプトが禁止している形式）。
var matchingReasonFormatPattern = regexp.MustCompile(`(?m)^\s*(#|[-*・●]|\d+[.)．])|[{}]`)

func newMatchingReasonTarget(modelOverride string) Target {
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
	return &matchingReasonTarget{model: m}
}

func (t *matchingReasonTarget) Name() string     { return TargetMatchingReason }
func (t *matchingReasonTarget) Model() string    { return t.model }
func (t *matchingReasonTarget) Endpoint() string { return "openai:/responses" }

func (t *matchingReasonTarget) EstimateTokens(c Case) (int, int) {
	// 上限到達でやり直す場合を含め、多めに見積もる
	return 2 * (ApproxTokensJA(c.InputText()) + matchingReasonPromptOverheadTokens),
		matchingReasonMaxOutputTokens + matchingReasonRetryMaxOutputTokens
}

func (t *matchingReasonTarget) Run(ctx context.Context, c Case) Observation {
	system, user := matching.MatchingReasonPrompts(c.Input.Match, c.Input.UserScores)
	first := CallResponses(ctx, t.model, system, user, matching.MatchingReasonTemperature, matchingReasonMaxOutputTokens, false)
	res, retried := first, false
	if first.Truncated && first.Err == nil {
		// 本番は max_output_tokens に当たると上限を上げて呼び直す
		res = CallResponses(ctx, t.model, system, user, matching.MatchingReasonTemperature, matchingReasonRetryMaxOutputTokens, false)
		res.PromptTokens += first.PromptTokens
		res.CompletionTokens += first.CompletionTokens
		res.LatencyMS += first.LatencyMS
		retried = true
	}
	o := evaluateMatchingReasonResponse(newObservation(c), c.Input, res)
	if retried {
		o.Violations = append(o.Violations, "retried_max_output_tokens")
	}
	return o
}

// evaluateMatchingReasonResponse は LLM の生出力を指標へ落とす（純粋関数）。
func evaluateMatchingReasonResponse(o Observation, in Input, res CallResult) Observation {
	o.LatencyMS = res.LatencyMS
	o.PromptTokens = res.PromptTokens
	o.CompletionTokens = res.CompletionTokens
	if res.Truncated {
		o.Broken, o.BrokenReason, o.Error = true, BrokenTruncated, "max_output_tokens に到達"
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
	reason := strings.TrimSpace(res.Text)
	if reason == "" {
		// 本番は空ならテンプレートの理由に戻す（AI の理由は使われない）
		o.Broken, o.BrokenReason, o.Error = true, BrokenCallFailed, "空の出力"
		return o
	}
	passed, total, violations := scoreMatchingReason(reason, in)
	o.Violations = append(o.Violations, violations...)
	o.RawScore = float64(passed)
	o.Score = float64(passed) / float64(total)
	return o
}

// scoreMatchingReason はプロンプトの指示のうち機械的に確かめられる5項目を数える。
func scoreMatchingReason(reason string, in Input) (passed, total int, violations []string) {
	check := func(name string, ok bool) {
		total++
		if ok {
			passed++
		} else {
			violations = append(violations, name)
		}
	}
	chars := runeLen(reason)
	check("length_out_of_range", chars >= matchingReasonMinChars && chars <= matchingReasonMaxChars)
	sentences := countSentences(reason)
	check("sentence_count", sentences >= 2 && sentences <= 3)
	check("no_company_fact", containsAny(reason, in.MustMentionAny))
	check("no_strength_axis", containsAny(reason, in.StrengthAny))
	check("format_violation", !matchingReasonFormatPattern.MatchString(reason))
	return passed, total, violations
}

// countSentences は句点（。！？）で区切った文の数。末尾に句点が無い文も1文と数える。
func countSentences(text string) int {
	n := 0
	last := ' '
	for _, r := range strings.TrimSpace(text) {
		if r == '。' || r == '！' || r == '？' || r == '!' || r == '?' {
			n++
		}
		last = r
	}
	if last != '。' && last != '！' && last != '？' && last != '!' && last != '?' && strings.TrimSpace(text) != "" {
		n++
	}
	return n
}

func containsAny(text string, words []string) bool {
	for _, w := range words {
		if w = strings.TrimSpace(w); w != "" && containsNormalized(text, w) {
			return true
		}
	}
	return false
}
