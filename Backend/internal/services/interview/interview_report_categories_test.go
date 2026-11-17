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
	long := strings.Repeat("あ", maxCategoryTextRunes+10)
	got := SanitizeCategoryBreakdown([]CategoryBreakdown{
		{Category: "逆質問", Score: 2, Weakness: "質問が1つだけ", Practice: "3つ用意する"},
		{Category: "知らないカテゴリ", Score: 3, Weakness: "x", Practice: "y"},
		{Category: " 自己紹介・志望動機 ", Score: 4, Weakness: "", Practice: long},
		{Category: "自己紹介・志望動機", Score: 1, Weakness: "重複", Practice: "重複"}, // 同じカテゴリは最初だけ
		{Category: "強み・弱み", Score: 6, Weakness: "範囲外", Practice: "範囲外"},
		{Category: "強み・弱み", Score: -1, Weakness: "範囲外", Practice: "範囲外"},
		{Category: "キャリアビジョン", Score: 3, Weakness: " ", Practice: ""}, // 文が空
		{Category: "企業別の質問", Score: 0, Weakness: "答えられなかった", Practice: "事業を調べる"},
	})
	var cats []string
	for _, c := range got {
		cats = append(cats, c.Category)
	}
	// 並びはカテゴリの定義順
	if want := []string{"自己紹介・志望動機", "逆質問", "企業別の質問"}; !slices.Equal(cats, want) {
		t.Fatalf("categories = %v, want %v", cats, want)
	}
	if got[0].Score != 4 || []rune(got[0].Practice)[0] != 'あ' || len([]rune(got[0].Practice)) != maxCategoryTextRunes {
		t.Fatalf("自己紹介 = %+v", got[0])
	}
	if got[2].Score != 0 {
		t.Fatalf("0点は残す: %+v", got[2])
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
