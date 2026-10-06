package aibench

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"Backend/internal/services/resume"
)

// resumeTarget は履歴書レビューを評価する。
//
// DB・S3・RAG には触らない。本番の ReviewDocument はドキュメント取得→PDF正規化→
// OCR→RAGレポート取得→LLM という流れだが、AI出力の品質を決めるのは最後の
// LLM 呼び出しだけで、前段は入力テキストを作る工程にすぎない。
// そこでゴールデンセットは OCR 後の平文を持ち、resume.BuildReviewPromptFromText で
// 本番と同一のプロンプトへ変換してから呼ぶ。
//
// companyInfo（RAGレポート・企業brief）は渡さない。企業情報は企業ごとに変わり、
// 固定できない入力を混ぜると「誰が測っても同じ数字」にならないため。
// 企業観点の評価が必要になったら、固定した企業briefをマニフェストへ足すこと。
type resumeTarget struct {
	model string
}

func newResumeTarget(modelOverride string) Target {
	m := strings.TrimSpace(modelOverride)
	if m == "" {
		m = resume.ReviewModel()
	}
	return &resumeTarget{model: m}
}

func (t *resumeTarget) Name() string     { return TargetResume }
func (t *resumeTarget) Model() string    { return t.model }
func (t *resumeTarget) Endpoint() string { return "openai:/responses" }

// resumePromptOverheadTokens はプロンプト定型文（指示＋ルーブリック＋出力例）の
// 概算トークン数。**本文を空にした BuildReviewPromptFromText を ApproxTokensJA に
// 通した実測値**で、#1584 でルーブリックに 0〜5 のレベル定義を足して
// 1153 → 2384 に増えた（約2.1倍）。
//
// ここを更新し忘れると事前の概算コストが過小になり、-yes を付けずに回したときの
// 確認画面が実費と合わなくなる。プロンプトかルーブリックを変えたら測り直すこと。
const resumePromptOverheadTokens = 2400

func (t *resumeTarget) EstimateTokens(c Case) (int, int) {
	// 出力は max_output_tokens を上限とする。
	return ApproxTokensJA(c.Input.ResumeText) + resumePromptOverheadTokens, resume.ReviewMaxOutputTokens
}

func (t *resumeTarget) Run(ctx context.Context, c Case) Observation {
	prompt := resume.BuildReviewPromptFromText(c.Input.ResumeText, c.Input.CompanyName, c.Input.JobTitle, "", c.Input.CandidateType)
	// 本番は JSON mode で呼ぶ（#1583）ので、ハーネスも true で揃える。
	res := CallResponses(ctx, t.model, resume.ReviewSystemPrompt, prompt, resume.ReviewTemperature, resume.ReviewMaxOutputTokens, true)
	return evaluateResumeResponse(newObservation(c), c, res)
}

// resumeReviewResponse はプロンプトが指定している出力形式。
//
// 本番の aiReviewResponse を借りずにハーネス側で定義しているのは、
// ここで検証したいのが「プロンプトに書いた契約を守っているか」であって
// 「本番のパーサが読めるか」ではないため。
//
// scores は #1529 のルーブリック（specificity / achievement / role_fit /
// completeness / readability、各0〜5）。総合点は LLM に出させず、
// 本番と同じ resume.ComputeResumeOverallScore で算出する。
type resumeReviewResponse struct {
	Scores  map[string]int     `json:"scores"`
	Summary string             `json:"summary"`
	Items   []resumeReviewItem `json:"items"`
}

type resumeReviewItem struct {
	Quote      string `json:"quote"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Severity   string `json:"severity"`
}

var resumeSeverities = []string{"info", "warning", "critical"}

// evaluateResumeResponse は LLM の生出力を指標へ落とす（純粋関数）。
func evaluateResumeResponse(o Observation, c Case, res CallResult) Observation {
	o.LatencyMS = res.LatencyMS
	o.PromptTokens = res.PromptTokens
	o.CompletionTokens = res.CompletionTokens

	// 上限到達を先に見る。途中で切れた出力は JSON としても壊れるが、
	// 直し方は「上限を上げる」であって「プロンプトを直す」ではない。
	// json_invalid に混ぜると対策を誤る。
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

	var resp resumeReviewResponse
	if err := json.Unmarshal([]byte(extractJSONObject(res.Text)), &resp); err != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenJSONInvalid, err.Error()
		return o
	}
	// 総合スコアは本番と同じ関数で算出する。ルーブリック違反なら本番も
	// スコアを捨てる（固定値を入れない / #1529）ので、ハーネスも
	// 「スコアが取れなかった」として破損に数える。
	//
	// 本番はルーブリック違反のとき別プロンプトで1度やり直す。ハーネスは
	// リトライを通さない方針なので、ここで出る schema_incomplete は
	// **やり直し前**の発生率である（README に明記）。
	candidateType := c.Input.CandidateType
	overall, scoreErr := resume.ComputeResumeOverallScore(resp.Scores, candidateType)
	if scoreErr != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenSchema, scoreErr.Error()
		return o
	}

	o.RawScore = float64(overall)
	o.Score = float64(overall) / 100

	if needsJSONRecovery(res.Text) {
		// プロンプトは「出力は次のJSONのみ」と指示している
		o.Violations = append(o.Violations, "json_not_bare")
	}
	if len(resp.Items) == 0 {
		o.Violations = append(o.Violations, "items_empty")
	}
	if len(resp.Items) > resume.ReviewMaxItems {
		o.Violations = append(o.Violations, "items_over_limit")
	}
	if strings.TrimSpace(resp.Summary) == "" {
		o.Violations = append(o.Violations, "summary_empty")
	}

	// 引用が本文に無ければ捏造。プロンプトは「必ず本文中に存在する短い引用」を
	// 要求しており、この quote は注釈PDFの位置合わせに使われる。
	// 本文に無い引用は位置合わせに失敗し、指摘そのものが学生に届かない。
	source := normalizeForQuote(c.Input.ResumeText)
	fabricated, missingFields, badSeverity := 0, 0, 0
	for _, it := range resp.Items {
		if strings.TrimSpace(it.Quote) == "" || strings.TrimSpace(it.Message) == "" || strings.TrimSpace(it.Suggestion) == "" {
			missingFields++
			continue
		}
		if !slices.Contains(resumeSeverities, it.Severity) {
			badSeverity++
		}
		if !strings.Contains(source, normalizeForQuote(it.Quote)) {
			fabricated++
		}
	}
	// 違反名に件数を埋め込まないこと。集計側（ViolationCounts）でキーが
	// 散らばり、「どの違反が何回起きたか」が数えられなくなる。
	if missingFields > 0 {
		o.Violations = append(o.Violations, "item_missing_fields")
	}
	if badSeverity > 0 {
		o.Violations = append(o.Violations, "severity_invalid")
	}
	if fabricated > 0 {
		o.Violations = append(o.Violations, "quote_not_in_text")
	}
	return o
}
