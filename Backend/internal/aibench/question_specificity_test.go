package aibench

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const questionSetPath = "../../../docs/research/ai-eval/interview-question.jsonl"

func loadQuestionSet(t *testing.T) []QuestionCase {
	t.Helper()
	cases, err := LoadQuestionCases(questionSetPath)
	if err != nil {
		t.Fatalf("ゴールデンセットを読めない: %v", err)
	}
	return cases
}

func findQuestionCase(t *testing.T, cases []QuestionCase, id string) QuestionCase {
	t.Helper()
	for _, c := range cases {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("ケース %s が無い", id)
	return QuestionCase{}
}

// ゴールデンセット自体の健全性。固有語が他社と被ると「固有語率」が意味を失う。
func Testゴールデンセットの検証(t *testing.T) {
	cases := loadQuestionSet(t)
	companies := map[string]bool{}
	for _, c := range cases {
		companies[c.CompanyName] = true
	}
	if len(companies) < 8 || len(companies) > 10 {
		t.Errorf("企業数 = %d, 8〜10社であること", len(companies))
	}
	ids := map[string]bool{}
	for _, c := range cases {
		if ids[c.ID] {
			t.Errorf("id重複: %s", c.ID)
		}
		ids[c.ID] = true
		if len(c.Facts) < 4 || len(c.TopAxes) == 0 || len(c.History) == 0 || c.Job == "" {
			t.Errorf("%s: 事実4件以上・上位軸・会話・職種が必要", c.ID)
		}
		factIDs := map[string]bool{}
		for _, f := range c.Facts {
			if factIDs[f.ID] {
				t.Errorf("%s: 事実ID重複 %s", c.ID, f.ID)
			}
			factIDs[f.ID] = true
			if len(f.Terms) == 0 {
				t.Errorf("%s/%s: 固有語が無い", c.ID, f.ID)
			}
			for _, term := range f.Terms {
				if !strings.Contains(f.Text, term) {
					t.Errorf("%s/%s: 固有語 %q が事実本文に無い", c.ID, f.ID, term)
				}
				// 他社の事実本文・社名に出る語は固有語にできない。
				for _, o := range cases {
					if o.CompanyName == c.CompanyName {
						continue
					}
					if strings.Contains(o.CompanyName, term) {
						t.Errorf("%s: 固有語 %q が %s の社名に含まれる", c.ID, term, o.ID)
					}
					for _, of := range o.Facts {
						if strings.Contains(of.Text, term) {
							t.Errorf("%s: 固有語 %q が %s の事実 %s に含まれる", c.ID, term, o.ID, of.ID)
						}
					}
				}
			}
		}
	}
}

func Test企業固有語率(t *testing.T) {
	cases := loadQuestionSet(t)
	minato := findQuestionCase(t, cases, "iq-01")
	tests := []struct {
		name string
		qs   []GeneratedQuestion
		want float64
	}{
		{"一般的な質問は0", []GeneratedQuestion{{Text: "あなたの強みを教えてください。"}, {Text: "チームで困った経験はありますか。"}}, 0},
		{"事実カード由来の語を含む質問は1", []GeneratedQuestion{{Text: "ポートゲートのような予約管理で、一番難しいのは何だと思いますか。"}, {Text: "Kubernetesでの運用に関心はありますか。"}}, 1},
		{"社名だけでも固有扱い（株式会社抜き）", []GeneratedQuestion{{Text: "ミナトロジスティクスを志望した理由は何ですか。"}}, 1},
		{"半分だけ固有なら0.5", []GeneratedQuestion{{Text: "ポートゲートで何を作りたいですか。"}, {Text: "最近嬉しかったことは何ですか。"}}, 0.5},
		{"質問なしは0（0除算しない）", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SpecificTermRate(minato, tt.qs); got != tt.want {
				t.Errorf("SpecificTermRate = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test他社の固有語では固有扱いにならない(t *testing.T) {
	cases := loadQuestionSet(t)
	minato := findQuestionCase(t, cases, "iq-01")
	// ハルカ農機の語を含む質問は、ミナトでは固有ではない。
	qs := []GeneratedQuestion{{Text: "ハルカ田植の開発に興味はありますか。"}}
	if got := SpecificTermRate(minato, qs); got != 0 {
		t.Errorf("他社の語で固有扱い: %v", got)
	}
}

func Test事実の実在検証(t *testing.T) {
	cases := loadQuestionSet(t)
	minato := findQuestionCase(t, cases, "iq-01")
	tests := []struct {
		name string
		q    GeneratedQuestion
		want []string
	}{
		{"実在するIDのみ", GeneratedQuestion{Text: "x", FactIDs: []string{"f1", "f3"}}, nil},
		{"引用なしは違反でない", GeneratedQuestion{Text: "x"}, nil},
		{"無い事実を引用したら違反", GeneratedQuestion{Text: "x", FactIDs: []string{"f1", "f99"}}, []string{"f99"}},
		{"空白のIDも無い事実", GeneratedQuestion{Text: "x", FactIDs: []string{" "}}, []string{" "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnknownFactIDs(minato, tt.q)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("UnknownFactIDs = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test差別的表現の検出(t *testing.T) {
	tests := []struct {
		name string
		q    string
		want bool
	}{
		{"家族構成", "ご家族の構成を教えてください。", true},
		{"親の職業", "お父様の職業は何ですか。", true},
		{"出身地", "ご出身はどちらですか。", true},
		{"本籍", "本籍はどこですか。", true},
		{"国籍", "国籍を教えてください。", true},
		{"宗教", "信仰している宗教はありますか。", true},
		{"支持政党", "支持する政党はありますか。", true},
		{"思想", "どのような思想をお持ちですか。", true},
		{"普通の質問", "学生時代に力を入れたことは何ですか。", false},
		{"出身大学は問題ない", "ご出身の大学ではどんな研究をしていましたか。", false},
		{"チームの家族的な雰囲気は問題ない", "家族のように支え合うチームで、あなたはどう動きますか。", false},
		{"政治の語を含むが思想を聞かない", "行政システムの開発で大切なことは何ですか。", false},
		{"企業固有の質問", "ポートゲートの障害対応で、どう動きますか。", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := len(DiscriminatoryHits(tt.q)) > 0
			if got != tt.want {
				t.Errorf("DiscriminatoryHits(%q) = %v, want %v", tt.q, DiscriminatoryHits(tt.q), tt.want)
			}
		})
	}
}

// fakeSwapJudge は「質問に他社の事情が書かれていなければ通用する」と判定する決定的なフェイク。
type fakeSwapJudge struct {
	calls int
	err   error
}

func (f *fakeSwapJudge) Applies(_ context.Context, q string, other QuestionCase) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	return !strings.Contains(q, "ポートゲート"), nil
}

func Test企業入替テスト(t *testing.T) {
	cases := loadQuestionSet(t)
	minato := findQuestionCase(t, cases, "iq-01")
	generic := GeneratedQuestion{Text: "あなたの強みを教えてください。"}
	specific := GeneratedQuestion{Text: "ポートゲートの予約管理で何を改善したいですか。"}

	j := &fakeSwapJudge{}
	got, err := GenericRate(context.Background(), cases, minato, []GeneratedQuestion{generic, specific}, j, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.5 {
		t.Errorf("一般的1・固有1で GenericRate = %v, want 0.5", got)
	}
	if j.calls != 4 {
		t.Errorf("判定回数 = %d, 質問2 × 他社2 = 4", j.calls)
	}

	// 入替先に自社は含まれない（同じ社名の別職種は他社扱いにしない）。
	got, _ = GenericRate(context.Background(), cases, minato, []GeneratedQuestion{generic}, &fakeSwapJudge{}, 50)
	if got != 1 {
		t.Errorf("全て一般的なら 1, got %v", got)
	}

	if _, err := GenericRate(context.Background(), cases, minato, []GeneratedQuestion{generic}, &fakeSwapJudge{err: errors.New("boom")}, 2); err == nil {
		t.Error("判定器のエラーを握りつぶしている")
	}
}

func Test有料判定器は明示の許可と上限が無いと作れない(t *testing.T) {
	ok := PaidJudgeOptions{Enabled: true, MaxCalls: 10, MaxUSD: 0.05, Model: "gpt-4o-mini"}
	tests := []struct {
		name   string
		apiKey string
		mut    func(*PaidJudgeOptions)
		wantOK bool
	}{
		{"全部そろう", "sk-test", func(*PaidJudgeOptions) {}, true},
		{"キー未設定", "", func(*PaidJudgeOptions) {}, false},
		{"フラグなし", "sk-test", func(o *PaidJudgeOptions) { o.Enabled = false }, false},
		{"回数上限なし", "sk-test", func(o *PaidJudgeOptions) { o.MaxCalls = 0 }, false},
		{"金額上限なし", "sk-test", func(o *PaidJudgeOptions) { o.MaxUSD = 0 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", tt.apiKey)
			o := ok
			tt.mut(&o)
			_, err := NewPaidSwapJudge(o)
			if (err == nil) != tt.wantOK {
				t.Errorf("err = %v, wantOK = %v", err, tt.wantOK)
			}
		})
	}
}

func Test有料判定器は上限で止まり超過後は呼ばない(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	cases := loadQuestionSet(t)
	calls := 0
	j, err := NewPaidSwapJudge(PaidJudgeOptions{Enabled: true, MaxCalls: 2, MaxUSD: 1, Model: "gpt-4o-mini"})
	if err != nil {
		t.Fatal(err)
	}
	j.call = func(context.Context, string, string, string) CallResult {
		calls++
		return CallResult{Text: `{"applies": true}`, PromptTokens: 500, CompletionTokens: 10}
	}
	for i := 0; i < 2; i++ {
		applies, err := j.Applies(context.Background(), "q", cases[1])
		if err != nil || !applies {
			t.Fatalf("%d回目: applies=%v err=%v", i, applies, err)
		}
	}
	if _, err := j.Applies(context.Background(), "q", cases[1]); !errors.Is(err, ErrJudgeBudget) {
		t.Errorf("回数上限超過で ErrJudgeBudget にならない: %v", err)
	}
	if calls != 2 {
		t.Errorf("上限超過後にAPIを呼んでいる: calls=%d", calls)
	}
}

func Test有料判定器は金額上限で止まる(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	cases := loadQuestionSet(t)
	j, _ := NewPaidSwapJudge(PaidJudgeOptions{Enabled: true, MaxCalls: 100, MaxUSD: 0.0001, Model: "gpt-4o-mini"})
	j.call = func(context.Context, string, string, string) CallResult {
		return CallResult{Text: `{"applies": false}`, PromptTokens: 100000, CompletionTokens: 100}
	}
	if _, err := j.Applies(context.Background(), "q", cases[1]); err != nil {
		t.Fatalf("1回目は事前見積もりが上限内なので通る: %v", err)
	}
	if _, err := j.Applies(context.Background(), "q", cases[1]); !errors.Is(err, ErrJudgeBudget) {
		t.Errorf("累計金額が上限を超えたら止まる: %v", err)
	}
}

func Test有料判定器は壊れた応答をエラーにする(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	cases := loadQuestionSet(t)
	j, _ := NewPaidSwapJudge(PaidJudgeOptions{Enabled: true, MaxCalls: 5, MaxUSD: 1, Model: "gpt-4o-mini"})
	j.call = func(context.Context, string, string, string) CallResult { return CallResult{Text: "はい"} }
	if _, err := j.Applies(context.Background(), "q", cases[1]); err == nil {
		t.Error("JSONでない応答を黙って false 扱いにしている")
	}
}
