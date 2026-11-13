package aibench

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"Backend/internal/services/costs"
)

// 面接の質問が「企業固有か、どの企業にも通用する一般的な質問か」を測る（#2274 段階0）。
//
// 質問の生成（services/interview）は呼ばない。生成済みの質問を受け取って測るだけで、
// 指標のうち LLM を使うのは企業入替テスト（SwapJudge）だけ。それ以外は決定的でCIで動く。

// QuestionFact は企業の事実カード1枚。
type QuestionFact struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"` // business / culture / work_style / tech / requirement
	Text  string   `json:"text"`
	Terms []string `json:"terms"` // この事実に由来する固有語（他社の事実には出ない語）
}

// QuestionTurn は面接の直前までの会話1発話。
type QuestionTurn struct {
	Role string `json:"role"` // Interviewer / User
	Text string `json:"text"`
}

// QuestionCase はゴールデンセット1行（合成の架空企業×職種）。
type QuestionCase struct {
	ID          string         `json:"id"`
	CompanyName string         `json:"company_name"`
	Job         string         `json:"job"`
	Facts       []QuestionFact `json:"facts"`
	TopAxes     []string       `json:"top_axes"`
	History     []QuestionTurn `json:"history"`
	Note        string         `json:"note"`
}

// GeneratedQuestion は測る対象の質問。FactIDs は質問が引用した事実カードのID。
type GeneratedQuestion struct {
	Text    string   `json:"text"`
	FactIDs []string `json:"fact_ids"`
}

// LoadQuestionCases は jsonl（1行1ケース）を読む。
func LoadQuestionCases(path string) ([]QuestionCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []QuestionCase
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var c QuestionCase
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// UniqueTerms は企業固有語（社名の「株式会社」抜き＋各事実の固有語）を返す。
func (c QuestionCase) UniqueTerms() []string {
	terms := []string{strings.TrimSpace(strings.TrimPrefix(c.CompanyName, "株式会社"))}
	for _, f := range c.Facts {
		terms = append(terms, f.Terms...)
	}
	return terms
}

// HasSpecificTerm は質問がその企業の固有語を1つでも含むかを返す。
func HasSpecificTerm(c QuestionCase, question string) bool {
	for _, t := range c.UniqueTerms() {
		if t != "" && strings.Contains(question, t) {
			return true
		}
	}
	return false
}

// SpecificTermRate は固有語を含む質問の割合（0〜1）。質問が無ければ0。
func SpecificTermRate(c QuestionCase, qs []GeneratedQuestion) float64 {
	if len(qs) == 0 {
		return 0
	}
	hit := 0
	for _, q := range qs {
		if HasSpecificTerm(c, q.Text) {
			hit++
		}
	}
	return float64(hit) / float64(len(qs))
}

// UnknownFactIDs は質問が引用したIDのうち、事実カードに無いものを返す（あれば違反）。
func UnknownFactIDs(c QuestionCase, q GeneratedQuestion) []string {
	known := make(map[string]bool, len(c.Facts))
	for _, f := range c.Facts {
		known[f.ID] = true
	}
	var unknown []string
	for _, id := range q.FactIDs {
		if !known[id] {
			unknown = append(unknown, id)
		}
	}
	return unknown
}

// 差別的表現（就職差別につながる事項）。家族・出身地・本籍・国籍・宗教・支持政党・思想。
// 「ご出身の大学」「家族のようなチーム」を誤検出しないよう、聞き方の形で絞る。
// ponytail: キーワード一致。言い換え（遠回しな聞き方）は拾えない。拾う必要が出たらLLM判定を足す。
var discriminatoryPatterns = []struct {
	category string
	re       *regexp.Regexp
}{
	{"家族", regexp.MustCompile(`家族構成|ご家族の(構成|職業|収入)|(お?父|お?母)(様|さん)?の(職業|仕事|勤務先)|両親の(職業|仕事)|親の(職業|仕事|年収)|兄弟(姉妹)?は何人`)},
	{"出身地", regexp.MustCompile(`出身(地|は(どちら|どこ))|ご出身はどちら|どこの出身|どこで生まれ|生まれ(は|た場所)|生まれ育った(地域|場所)`)},
	{"本籍", regexp.MustCompile(`本籍`)},
	{"国籍", regexp.MustCompile(`国籍|日本人ですか`)},
	{"宗教", regexp.MustCompile(`宗教|信仰|信じている神|宗派`)},
	{"支持政党", regexp.MustCompile(`支持(する|している)?政党|投票(する|した)(政党|先)|どの政党`)},
	{"思想", regexp.MustCompile(`思想|信条|政治的(な)?(立場|主張)`)},
}

// DiscriminatoryHits は質問に含まれる差別的表現のカテゴリを返す（無ければ空）。
func DiscriminatoryHits(question string) []string {
	var hits []string
	for _, p := range discriminatoryPatterns {
		if p.re.MatchString(question) {
			hits = append(hits, p.category)
		}
	}
	return hits
}

// SwapJudge は「この質問を別の企業の面接に置いても自然に通用するか」を判定する。
// テストはフェイク、本番は PaidSwapJudge（有料）。
type SwapJudge interface {
	Applies(ctx context.Context, question string, other QuestionCase) (bool, error)
}

// swapTargets は c とは別の企業を、ケース順の並びで c の次から最大 n 社返す（決定的）。
func swapTargets(cases []QuestionCase, c QuestionCase, n int) []QuestionCase {
	start := 0
	for i, x := range cases {
		if x.ID == c.ID {
			start = i
			break
		}
	}
	seen := map[string]bool{c.CompanyName: true}
	var out []QuestionCase
	for k := 1; k <= len(cases) && len(out) < n; k++ {
		x := cases[(start+k)%len(cases)]
		if seen[x.CompanyName] {
			continue
		}
		seen[x.CompanyName] = true
		out = append(out, x)
	}
	return out
}

// GenericRate は企業入替テストの「一般的」判定率（0〜1。高いほど一般的な質問が多い）。
// 各質問を別の企業 swapN 社に置き、通用すると判定された割合。
func GenericRate(ctx context.Context, cases []QuestionCase, c QuestionCase, qs []GeneratedQuestion, j SwapJudge, swapN int) (float64, error) {
	targets := swapTargets(cases, c, swapN)
	total, generic := 0, 0
	for _, q := range qs {
		for _, o := range targets {
			ok, err := j.Applies(ctx, q.Text, o)
			if err != nil {
				return 0, fmt.Errorf("%s: 企業入替の判定に失敗: %w", c.ID, err)
			}
			total++
			if ok {
				generic++
			}
		}
	}
	if total == 0 {
		return 0, nil
	}
	return float64(generic) / float64(total), nil
}

// --- 有料の判定器 -------------------------------------------------------

// ErrJudgeBudget は呼び出し回数または金額の上限に達したことを示す。
var ErrJudgeBudget = errors.New("有料判定器の上限（呼び出し回数または金額）に達しました")

// PaidJudgeOptions は有料判定器の明示の許可と上限。どれかが欠けると作れない。
type PaidJudgeOptions struct {
	Enabled  bool    // 有料実行の明示フラグ
	MaxCalls int     // 呼び出し回数の上限（1以上）
	MaxUSD   float64 // 概算金額の上限（USD、0より大きい）
	Model    string  // 既定 gpt-4o-mini
}

// PaidSwapJudge は OpenAI で企業入替を判定する。既定では使われない。
type PaidSwapJudge struct {
	opt PaidJudgeOptions

	mu    sync.Mutex
	calls int
	spent float64
	call  func(ctx context.Context, model, system, user string) CallResult // テストで差し替える
}

// NewPaidSwapJudge は OPENAI_API_KEY・Enabled・両方の上限がそろわないとエラーにする。
func NewPaidSwapJudge(opt PaidJudgeOptions) (*PaidSwapJudge, error) {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return nil, errors.New("OPENAI_API_KEY が未設定です（有料判定は実行しません）")
	}
	if !opt.Enabled {
		return nil, errors.New("有料判定が明示的に有効化されていません（Enabled=true が必要）")
	}
	if opt.MaxCalls < 1 || opt.MaxUSD <= 0 {
		return nil, errors.New("呼び出し回数の上限と金額の上限が必要です（MaxCalls>=1, MaxUSD>0）")
	}
	if strings.TrimSpace(opt.Model) == "" {
		opt.Model = "gpt-4o-mini"
	}
	return &PaidSwapJudge{opt: opt, call: func(ctx context.Context, model, system, user string) CallResult {
		return CallChatCompletions(ctx, model, system, user, 0, 40, true)
	}}, nil
}

const swapJudgeSystemPrompt = `あなたは面接の質問を評価する審査員です。
面接官の質問を、示された「別の企業」の面接でそのまま使っても自然に通用するかを判定します。
その企業の事業・文化・働き方・技術・求人要件のいずれにも結びつかず、どの企業でも聞ける質問なら applies は true。
その企業の事実に基づかない限り成立しない質問なら false。
JSONだけを返してください: {"applies": true または false}`

func swapJudgeUserPrompt(question string, o QuestionCase) string {
	var b strings.Builder
	fmt.Fprintf(&b, "別の企業: %s（%s）\n", o.CompanyName, o.Job)
	for _, f := range o.Facts {
		fmt.Fprintf(&b, "- [%s] %s\n", f.Kind, f.Text)
	}
	fmt.Fprintf(&b, "\n質問: %s", question)
	return b.String()
}

// Applies は上限内なら1回だけ呼ぶ。事前に見積もりが上限を超えるなら呼ばない。
func (p *PaidSwapJudge) Applies(ctx context.Context, question string, other QuestionCase) (bool, error) {
	user := swapJudgeUserPrompt(question, other)
	estPrompt := ApproxTokensJA(swapJudgeSystemPrompt + user)
	est := costs.EstimateCostUSD(p.opt.Model, estPrompt, 30)

	p.mu.Lock()
	if p.calls >= p.opt.MaxCalls || p.spent+est > p.opt.MaxUSD {
		p.mu.Unlock()
		return false, ErrJudgeBudget
	}
	p.calls++
	p.spent += est // 先に見積もりを計上し、並行呼び出しでも上限を超えない
	p.mu.Unlock()

	res := p.call(ctx, p.opt.Model, swapJudgeSystemPrompt, user)

	p.mu.Lock()
	if res.PromptTokens > 0 || res.CompletionTokens > 0 {
		p.spent += costs.EstimateCostUSD(p.opt.Model, res.PromptTokens, res.CompletionTokens) - est
	}
	p.mu.Unlock()

	if res.Err != nil {
		return false, res.Err
	}
	var out struct {
		Applies *bool `json:"applies"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(res.Text)), &out); err != nil || out.Applies == nil {
		return false, fmt.Errorf("判定の応答を読めません: %q", snippet([]byte(res.Text)))
	}
	return *out.Applies, nil
}

// SpentUSD は概算の累計金額（実トークンが取れたものは実測ベース）。
func (p *PaidSwapJudge) SpentUSD() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.spent
}
