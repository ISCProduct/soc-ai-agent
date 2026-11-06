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
// 概算トークン数。**本文1文字の BuildReviewPromptFromText を ApproxTokensJA に
// 通し、本文の1文字を引いた実測値**で、#1584 でルーブリックに 0〜5 のレベル定義を足して
// 1153 → 2384 に増えた（約2.1倍）。
// #1600 で応募情報とOCR本文をノンス付き区切りで囲んだ分（宣言文2つ）がさらに乗り、
// 実測 2685 になった。#1942 で rewrite の契約と出力例を足して実測 2829 になった。
// 本文を空にして測ると WrapUntrustedText が空を返して
// 宣言文が乗らず、約136トークン過小になる（#1644）。
//
// ここを更新し忘れると事前の概算コストが過小になり、-yes を付けずに回したときの
// 確認画面が実費と合わなくなる。TestResumePromptOverheadTokens_MatchesPrompt が検出する。
const resumePromptOverheadTokens = 2829

func (t *resumeTarget) EstimateTokens(c Case) (int, int) {
	// 出力は max_output_tokens を上限とする。
	return ApproxTokensJA(c.Input.ResumeText) + resumePromptOverheadTokens, resume.ReviewMaxOutputTokens
}

func (t *resumeTarget) Run(ctx context.Context, c Case) Observation {
	prompt := resume.BuildReviewPromptFromTextWithConfidence(c.Input.ResumeText, c.Input.OCRLineConfidence, c.Input.CompanyName, c.Input.JobTitle, "", c.Input.CandidateType)
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
	// Rewrite は quote をそのまま置き換える一文（#1942）。空は欠落として数える。
	// 低信頼行では空が正しく、文が入っている方が違反。
	Rewrite  string `json:"rewrite"`
	Severity string `json:"severity"`
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
	missingRewrite, lowRewrite, sameRewrite := 0, 0, 0
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
		// 低信頼行への書き換えは、本文に引用が無くても別に数える。
		// プロンプトは「空文字にせよ」と要求しており、捏造とは直し方が違う。
		rewrite := strings.TrimSpace(it.Rewrite)
		low := quoteOnLowConfidenceLine(it.Quote, c.Input.ResumeText, c.Input.OCRLineConfidence)
		switch {
		case rewrite == "" && !low:
			missingRewrite++
		case rewrite != "" && low:
			lowRewrite++
		case rewrite != "" && normalizeForQuote(rewrite) == normalizeForQuote(it.Quote):
			sameRewrite++
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
	if missingRewrite > 0 {
		o.Violations = append(o.Violations, "rewrite_missing")
	}
	if lowRewrite > 0 {
		o.Violations = append(o.Violations, "rewrite_on_low_confidence")
	}
	if sameRewrite > 0 {
		o.Violations = append(o.Violations, "rewrite_same_as_quote")
	}
	return o
}

// quoteOnLowConfidenceLine は、引用が認識スコア下限未満の行に当たるかを返す。
//
// スコアが無い・範囲外の行は不明なので当たらない（本番が書き換えを禁止しないのと同じ）。
// 行が引用を含む、または6文字以上の行が引用に含まれるとき当たったとみなす。
// 短い行の包含は表の見出しが長い引用に偶然入るのを避けるため見ない。
// 引用が複数行にまたがり、低信頼行の一部だけを含む場合も当たったとみなす。
// 行全体の包含だけだと、隣の誤読を少しだけ取り込んだ引用を見逃す。
func quoteOnLowConfidenceLine(quote, resumeText string, confidence []float64) bool {
	if len(confidence) == 0 {
		return false
	}
	normQuote := normalizeForQuote(quote)
	if normQuote == "" {
		return false
	}
	type row struct {
		norm string
		low  bool
	}
	var rows []row
	i := 0
	for _, line := range strings.Split(resumeText, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var score float64
		known := false
		if i < len(confidence) {
			score = confidence[i]
			known = score >= 0 && score <= 1
		}
		i++
		normLine := normalizeForQuote(line)
		low := known && score < resume.OCRRewriteMinConfidence
		rows = append(rows, row{norm: normLine, low: low})
		if !low || normLine == "" {
			continue
		}
		if strings.Contains(normLine, normQuote) {
			return true
		}
		if len([]rune(normLine)) >= 6 && strings.Contains(normQuote, normLine) {
			return true
		}
	}
	// 本番の照合窓と同じく、連続する最大4行を見る。
	for size := 2; size <= 4; size++ {
		for start := 0; start+size <= len(rows); start++ {
			var b strings.Builder
			low := false
			for _, row := range rows[start : start+size] {
				b.WriteString(row.norm)
				if row.low {
					low = true
				}
			}
			if !low {
				continue
			}
			joined := b.String()
			if joined == "" || (!strings.Contains(joined, normQuote) && !strings.Contains(normQuote, joined)) {
				continue
			}
			for _, row := range rows[start : start+size] {
				if row.low && lineTouchesQuote(normQuote, row.norm) {
					return true
				}
			}
		}
	}
	return false
}

// lineTouchesQuote は、低信頼行の6文字以上が引用に入っているかを返す。
// 行全体が引用に含まれない折り返しでも、誤読部分を拾う。
func lineTouchesQuote(normQuote, normLine string) bool {
	if normLine == "" {
		return false
	}
	if strings.Contains(normQuote, normLine) || strings.Contains(normLine, normQuote) {
		return true
	}
	runes := []rune(normLine)
	if len(runes) < 6 {
		return false
	}
	for i := 0; i+6 <= len(runes); i++ {
		if strings.Contains(normQuote, string(runes[i:i+6])) {
			return true
		}
	}
	return false
}
