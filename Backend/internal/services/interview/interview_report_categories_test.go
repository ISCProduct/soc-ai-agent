package interview

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// SOCAIAGENT-610: 質問カテゴリ別の弱点。

// スキーマの enum とカテゴリの定義がずれると、LLM が出せないカテゴリができるか、捨てられるカテゴリが出る。
func TestCategoryBreakdown_スキーマのenumとカテゴリが一致する(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			Items struct {
				Properties map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"items"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(interviewReportResponseSchema.Schema, &schema); err != nil {
		t.Fatal(err)
	}
	got := schema.Properties["category_breakdown"].Items.Properties["category"].Enum
	if !slices.Equal(got, ReportQuestionCategories) {
		t.Fatalf("enum = %v, want %v", got, ReportQuestionCategories)
	}
	if !slices.Contains(schema.Required, "category_breakdown") {
		t.Fatal("category_breakdown が required に無い（strict のスキーマでは全項目が必須）")
	}
}

func TestBuildReportPrompts_カテゴリ別の指示を含む(t *testing.T) {
	_, user := BuildReportPrompts("ja", "User [utterance_id=1]: 志望理由は…")
	for _, want := range []string{"category_breakdown", "企業別の質問", "質問されなかったカテゴリは含めない", "0〜5"} {
		if !strings.Contains(user, want) {
			t.Errorf("プロンプトに %q が無い", want)
		}
	}
	// 指示は信頼領域（面接ログの囲みより前）に置く
	if strings.Index(user, "category_breakdown") > strings.Index(user, "Interview transcript:") {
		t.Fatal("カテゴリ別の指示が面接ログより後ろにある")
	}
}

func TestSanitizeCategoryBreakdown(t *testing.T) {
	exact := strings.Repeat("あ", maxCategoryTextRunes)
	long := strings.Repeat("あ", maxCategoryTextRunes+10)
	tests := []struct {
		name string
		in   []CategoryBreakdown
		want []CategoryBreakdown
	}{
		{
			name: "並びはカテゴリの定義順。前後の空白は取る",
			in: []CategoryBreakdown{
				{Category: "逆質問", Score: 2, Weakness: "質問が1つだけ", Practice: "3つ用意する"},
				{Category: " 自己紹介・志望動機 ", Score: 4, Weakness: " 弱 ", Practice: "練"},
			},
			want: []CategoryBreakdown{
				{Category: "自己紹介・志望動機", Score: 4, Weakness: "弱", Practice: "練"},
				{Category: "逆質問", Score: 2, Weakness: "質問が1つだけ", Practice: "3つ用意する"},
			},
		},
		{
			name: "知らないカテゴリは捨てる",
			in:   []CategoryBreakdown{{Category: "知らないカテゴリ", Score: 3, Weakness: "x", Practice: "y"}},
			want: nil,
		},
		{
			name: "同じカテゴリは最初の1件だけ",
			in: []CategoryBreakdown{
				{Category: "強み・弱み", Score: 3, Weakness: "1件目", Practice: ""},
				{Category: "強み・弱み", Score: 1, Weakness: "重複", Practice: "重複"},
			},
			want: []CategoryBreakdown{{Category: "強み・弱み", Score: 3, Weakness: "1件目"}},
		},
		{
			name: "0点と5点は残す",
			in: []CategoryBreakdown{
				{Category: "企業別の質問", Score: RubricScoreMin, Weakness: "答えられなかった", Practice: "事業を調べる"},
				{Category: "キャリアビジョン", Score: RubricScoreMax, Weakness: "", Practice: "この調子で"},
			},
			want: []CategoryBreakdown{
				{Category: "キャリアビジョン", Score: RubricScoreMax, Practice: "この調子で"},
				{Category: "企業別の質問", Score: RubricScoreMin, Weakness: "答えられなかった", Practice: "事業を調べる"},
			},
		},
		{
			name: "6点と-1点は捨てる",
			in: []CategoryBreakdown{
				{Category: "強み・弱み", Score: RubricScoreMax + 1, Weakness: "範囲外"},
				{Category: "逆質問", Score: RubricScoreMin - 1, Weakness: "範囲外"},
			},
			want: nil,
		},
		{
			// 文が空という理由で比較から消すと、「次に練習したい」が本当に低い種類でなくなる
			name: "文が両方空でも、点数が範囲内なら行は残す",
			in: []CategoryBreakdown{
				{Category: "自己紹介・志望動機", Score: 1, Weakness: " ", Practice: ""},
				{Category: "職務経験・実績", Score: 3, Weakness: "w", Practice: "p"},
			},
			want: []CategoryBreakdown{
				{Category: "自己紹介・志望動機", Score: 1},
				{Category: "職務経験・実績", Score: 3, Weakness: "w", Practice: "p"},
			},
		},
		{
			name: "上限ちょうどの文は切らない",
			in:   []CategoryBreakdown{{Category: "逆質問", Score: 2, Weakness: exact}},
			want: []CategoryBreakdown{{Category: "逆質問", Score: 2, Weakness: exact}},
		},
		{
			name: "上限を超える文は、省略記号を含めて上限の文字数に切る",
			in:   []CategoryBreakdown{{Category: "逆質問", Score: 2, Practice: long}},
			want: []CategoryBreakdown{{Category: "逆質問", Score: 2, Practice: strings.Repeat("あ", maxCategoryTextRunes-1) + "…"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCategoryBreakdown(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got  = %+v\nwant = %+v", got, tt.want)
			}
		})
	}
}

func TestSanitizeCategoryBreakdown_残らなければnil(t *testing.T) {
	if got := SanitizeCategoryBreakdown(nil); got != nil {
		t.Fatalf("got = %v", got)
	}
	if got := SanitizeCategoryBreakdown([]CategoryBreakdown{{Category: "不明", Score: 3, Weakness: "x"}}); got != nil {
		t.Fatalf("got = %v", got)
	}
	if categoryBreakdownJSON(nil) != nil {
		t.Fatal("内訳が無ければ NULL で保存する（JSON カラムに空文字は書けない）")
	}
}

func TestBuildReportRecord_カテゴリ別の内訳を保存する(t *testing.T) {
	s := &InterviewService{}
	rec := s.buildReportRecord(1, reportPayload{
		Summary: "要約",
		CategoryBreakdown: []CategoryBreakdown{
			{Category: "逆質問", Score: 2, Weakness: "質問が1つだけ", Practice: "3つ用意する"},
			{Category: "不明", Score: 3, Weakness: "x", Practice: "y"},
		},
	})
	if rec.CategoryBreakdownJSON == nil {
		t.Fatal("内訳が保存されていない")
	}
	var saved []CategoryBreakdown
	if err := json.Unmarshal([]byte(*rec.CategoryBreakdownJSON), &saved); err != nil || len(saved) != 1 || saved[0].Category != "逆質問" {
		t.Fatalf("saved = %+v err = %v", saved, err)
	}
	if rec := s.buildReportRecord(1, reportPayload{Summary: "要約"}); rec.CategoryBreakdownJSON != nil {
		t.Fatal("内訳が無いレポートは NULL")
	}
}
