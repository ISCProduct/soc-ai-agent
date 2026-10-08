package aibench

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/interview"
)

// interviewTarget は面接レポート生成を評価する。
//
// DB には触らない。本番の generateReport はセッションと発話を DB から読むが、
// LLM へ渡るのは BuildTranscript の結果（文字列）だけなので、ゴールデンセットは
// その文字列をそのまま持つ。プロンプトは interview.BuildReportPrompts を使い、
// 本番と同一にする。
type interviewTarget struct {
	model string
}

// 本番の呼び出しパラメータ（generateReport と同じ値）。
// ここを本番と違う値にすると、破損率も再現性も別物を測ることになる。
const (
	interviewTemperature = 0.4
	interviewMaxTokens   = 2000
)

func newInterviewTarget(modelOverride string) Target {
	// 本番は INTERVIEW_REPORT_MODEL 未設定なら openai.Client の DefaultModel に
	// 委ねるため、ハーネスでは明示的な既定を持つ（結果にモデル名を残すため）。
	m := strings.TrimSpace(modelOverride)
	if m == "" {
		m = strings.TrimSpace(os.Getenv("INTERVIEW_REPORT_MODEL"))
	}
	if m == "" {
		m = "gpt-4o-mini"
	}
	return &interviewTarget{model: m}
}

func (t *interviewTarget) Name() string     { return TargetInterviewReport }
func (t *interviewTarget) Model() string    { return t.model }
func (t *interviewTarget) Endpoint() string { return "openai:/chat/completions" }

func (t *interviewTarget) EstimateTokens(c Case) (int, int) {
	// 1500 はルーブリック説明と出力フォーマット指定ぶんの概算。
	return ApproxTokensJA(c.Input.Transcript) + 1500, interviewMaxTokens
}

func (t *interviewTarget) Run(ctx context.Context, c Case) Observation {
	lang := c.Input.Lang
	if lang == "" {
		lang = "ja"
	}
	systemPrompt, userPrompt := interview.BuildReportPrompts(lang, c.Input.Transcript)
	res := CallChatCompletions(ctx, t.model, systemPrompt, userPrompt, interviewTemperature, interviewMaxTokens, true)
	return evaluateInterviewResponse(newObservation(c), c, res)
}

// interviewReportResponse はプロンプトが指定している出力形式。
type interviewReportResponse struct {
	Summary      string            `json:"summary"`
	Scores       map[string]int    `json:"scores"`
	Evidence     map[string]string `json:"evidence"`
	Strengths    []string          `json:"strengths"`
	Improvements []string          `json:"improvements"`
	Teacher      *struct {
		OverallComment   string            `json:"overall_comment"`
		DetailedEvidence map[string]string `json:"detailed_evidence"`
		CoachingPoints   []string          `json:"coaching_points"`
	} `json:"teacher"`
}

// strengths / improvements はプロンプトで「各2〜4件」と指示している。
const (
	interviewListMin = 2
	interviewListMax = 4
)

// evaluateInterviewResponse は LLM の生出力を指標へ落とす（純粋関数）。
func evaluateInterviewResponse(o Observation, c Case, res CallResult) Observation {
	o.LatencyMS = res.LatencyMS
	o.PromptTokens = res.PromptTokens
	o.CompletionTokens = res.CompletionTokens

	if res.Truncated {
		o.Broken, o.BrokenReason, o.Error = true, BrokenTruncated, "max_completion_tokens に到達"
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

	var resp interviewReportResponse
	if err := json.Unmarshal([]byte(extractJSONObject(res.Text)), &resp); err != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenJSONInvalid, err.Error()
		return o
	}
	if len(resp.Scores) == 0 {
		o.Broken, o.BrokenReason, o.Error = true, BrokenSchema, "scores が無い"
		return o
	}

	// スコアはルーブリックの全項目の平均。欠けている項目は0として平均に含める。
	// 欠落を無視して平均すると、1項目しか返さないモデルが高得点になる。
	keys := interview.RubricKeys()
	var sum float64
	for _, k := range keys {
		sum += float64(resp.Scores[k])
	}
	o.RawScore = sum / float64(len(keys))
	o.Score = o.RawScore / float64(interview.RubricScoreMax)

	if needsJSONRecovery(res.Text) {
		o.Violations = append(o.Violations, "json_not_bare")
	}
	// 値域・キー欠落は本番と同じ検証を通す（別実装にすると判定がずれる）
	if err := interview.ValidateRubricScores(resp.Scores); err != nil {
		o.Violations = append(o.Violations, "scores_invalid")
	}
	if n := len(resp.Strengths); n < interviewListMin || n > interviewListMax {
		o.Violations = append(o.Violations, "strengths_count")
	}
	if n := len(resp.Improvements); n < interviewListMin || n > interviewListMax {
		o.Violations = append(o.Violations, "improvements_count")
	}
	if strings.TrimSpace(resp.Summary) == "" {
		o.Violations = append(o.Violations, "summary_empty")
	}
	if resp.Teacher == nil || strings.TrimSpace(resp.Teacher.OverallComment) == "" {
		o.Violations = append(o.Violations, "teacher_missing")
	}

	// 根拠の捏造。プロンプトは「受験者が実際に話した発言をそのまま引用」を
	// 指示しており、本番も同じ照合で弾いている（#1527 / #1566）。
	// ハーネス側で別の照合を書くと、本番が弾く/弾かないの境界と数字がずれる。
	// #1580 で照合が内容語ベース（SpokenContent）になっている。
	spoken := interview.SpokenContent(utterancesFromTranscript(c.Input.Transcript))
	if check := interview.ValidateEvidence(resp.Evidence, spoken); len(check.Unmatched) > 0 {
		o.Violations = append(o.Violations, "evidence_not_spoken")
	}
	if resp.Teacher != nil {
		if check := interview.ValidateEvidence(resp.Teacher.DetailedEvidence, spoken); len(check.Unmatched) > 0 {
			o.Violations = append(o.Violations, "teacher_evidence_not_spoken")
		}
	}
	return o
}

// utterancesFromTranscript は "User: ..." / "Interviewer: ..." 形式の
// トランスクリプトを発話へ戻す（interview.BuildTranscript の逆）。
//
// 根拠の照合は受験者(role=user)の発話だけを対象にするため、役割の復元が必要。
// 行頭のラベルが無い行は直前の発話の続きとして扱う（複数行の発話に対応）。
func utterancesFromTranscript(transcript string) []models.InterviewUtterance {
	var out []models.InterviewUtterance
	for _, line := range strings.Split(transcript, "\n") {
		switch {
		case strings.HasPrefix(line, "User: "):
			out = append(out, models.InterviewUtterance{Role: "user", Text: strings.TrimPrefix(line, "User: ")})
		case strings.HasPrefix(line, "Interviewer: "):
			out = append(out, models.InterviewUtterance{Role: "ai", Text: strings.TrimPrefix(line, "Interviewer: ")})
		default:
			if strings.TrimSpace(line) == "" || len(out) == 0 {
				continue
			}
			out[len(out)-1].Text += "\n" + line
		}
	}
	return out
}
