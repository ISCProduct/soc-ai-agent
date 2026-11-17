package interview

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// 質問カテゴリ別の弱点（SOCAIAGENT-610 / #2025）。
//
// 面接レポートの評価はルーブリック5軸（論理性など）だけで、どの種類の質問が弱いかが分からなかった。
// レポートを作る LLM 呼び出し（1回）の中で、面接官の質問をカテゴリに分け、カテゴリごとに
// 同じ 0〜5 の基準で採点し、弱点と次に練習することを1文ずつ書かせる（呼び出しは増やさない）。

// companyQuestionCategory は企業から指定された質問・企業研究に基づく質問のカテゴリ。
const companyQuestionCategory = "企業別の質問"

// ReportQuestionCategories はカテゴリ別の内訳で使うカテゴリ。並びは面接の進め方（interviewTopics）の順で、
// 企業別の質問を最後に置く。スキーマの enum もこれと同じ（テストで確かめる）。
var ReportQuestionCategories = append(append([]string{}, interviewTopics...), companyQuestionCategory)

// maxCategoryTextRunes は弱点・練習の文の上限（長すぎる生成を画面に出さない）。
const maxCategoryTextRunes = 200

// CategoryBreakdown は質問カテゴリ1つの評価。
type CategoryBreakdown struct {
	Category string `json:"category"`
	Score    int    `json:"score"`
	Weakness string `json:"weakness"`
	Practice string `json:"practice"`
}

// SanitizeCategoryBreakdown は LLM の出力を、保存してよい内訳にそろえる。
//
// 知らないカテゴリ・範囲外の点数・空の文の行は捨て、同じカテゴリは最初の1件だけ残す。
// 並びはカテゴリの定義順。1件も残らなければ nil（レポート本体は保存する。内訳だけ出さない）。
func SanitizeCategoryBreakdown(items []CategoryBreakdown) []CategoryBreakdown {
	order := make(map[string]int, len(ReportQuestionCategories))
	for i, c := range ReportQuestionCategories {
		order[c] = i
	}
	byCategory := map[string]CategoryBreakdown{}
	for _, it := range items {
		it.Category = strings.TrimSpace(it.Category)
		if _, ok := order[it.Category]; !ok {
			continue
		}
		if _, dup := byCategory[it.Category]; dup {
			continue
		}
		if it.Score < RubricScoreMin || it.Score > RubricScoreMax {
			continue
		}
		it.Weakness = truncateRunes(strings.TrimSpace(it.Weakness), maxCategoryTextRunes)
		it.Practice = truncateRunes(strings.TrimSpace(it.Practice), maxCategoryTextRunes)
		if it.Weakness == "" && it.Practice == "" {
			continue
		}
		byCategory[it.Category] = it
	}
	if len(byCategory) == 0 {
		return nil
	}
	out := make([]CategoryBreakdown, 0, len(byCategory))
	for _, c := range ReportQuestionCategories {
		if it, ok := byCategory[c]; ok {
			out = append(out, it)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// categoryBreakdownJSON は保存用の JSON。内訳が無ければ nil（JSON カラムに空文字は書けない）。
func categoryBreakdownJSON(items []CategoryBreakdown) *string {
	if len(items) == 0 {
		return nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

// categoryBreakdownPromptSection はレポート生成プロンプトに足す、カテゴリ別の評価の指示。
func categoryBreakdownPromptSection() string {
	return `## 質問カテゴリ別の評価（category_breakdown）
面接官（Interviewer）の各質問を、次のカテゴリのどれか1つに分けてください: ` + strings.Join(ReportQuestionCategories, "、") + `
- 企業から指定された質問や、企業の事業・研究に基づく質問は「` + companyQuestionCategory + `」にしてください
- 実際に質問されたカテゴリだけを出力してください（質問されなかったカテゴリは含めない）
- score は、そのカテゴリの質問への受験者の回答を、上の評価基準と同じ 0〜5 で採点してください
- weakness は、そのカテゴリの回答で足りなかった点を1文で（生徒向けのやさしい言葉で。良かった場合は空文字でよい）
- practice は、次の練習で意識することを1文で具体的に`
}
