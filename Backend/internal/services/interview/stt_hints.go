package interview

import (
	"regexp"
	"sort"
	"strings"
)

// BuildSTTHints は面接コンテキストから音声認識の補助語を組み立てる（音声R&D Task 4）。
//
// Transcription API の prompt に渡す。実測では固有名詞の認識が改善し、
// 無関係な語を渡しても幻覚は起きなかった（RESULTS_stt_hints.md）。
//
// 特に「御社」は mini モデルが補助語なしだと8回中0回しか正しく取れず、
// 全て「本社」になった。面接では意味が変わるため、常に含める。
//
// 空でも従来どおり動くこと。企業未選択の面接でも面接は成立する。
// skillScores は補助語に使わない。models.SkillScore が持つのは
// カテゴリ（論理性・技術志向など）とスコアだけで、
// 「Go」「AWS」のような固有の技術名を持たないため補助にならない。
func BuildSTTHints(companyName, companyReading, position, companyInfo string) string {
	seen := map[string]bool{}
	hints := make([]string, 0, 16)

	add := func(s string) {
		s = strings.TrimSpace(s)
		// 1文字の語は誤検出を招くだけで補助にならない
		if len([]rune(s)) < 2 || seen[s] {
			return
		}
		seen[s] = true
		hints = append(hints, s)
	}

	// 面接で必ず出る語。モデルが「本社」と取り違えるため最優先で入れる。
	add("御社")

	add(companyName)
	add(companyReading)
	add(position)

	// 企業情報からは技術用語だけを拾う。文章をそのまま渡すと
	// 補助語ではなく「続きの文脈」として扱われ、認識が引きずられる。
	for _, t := range extractTechTerms(companyInfo) {
		add(t)
	}
	// 語数の上限は extractTechTerms 側（MaxTechTerms）で決まる。
	// 「御社」+ 企業名・読み・職種の3語 + 技術用語8語で最大12語。
	// ここに二重の上限を置いてもテストで到達できず、
	// 効いているのか分からないコードになるため置かない。
	return strings.Join(hints, ", ")
}

// MaxTechTerms は企業情報から抽出する技術用語の上限。
// 長すぎるpromptは効果が薄れ、費用も増える。実測では10語前後で十分な改善が出た。
const MaxTechTerms = 8

// techTermPattern は英字の技術用語・製品名。
// 日本語の一般語まで拾うと補助語がノイズだらけになる。
var techTermPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+#.]{1,19}`)

// commonEnglishWords は技術用語として扱わない語。
// 企業紹介文に頻出するが、補助語としての価値が無い。
var commonEnglishWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "our": true,
	"we": true, "you": true, "are": true, "com": true, "http": true,
	"https": true, "www": true,
}

func extractTechTerms(companyInfo string) []string {
	if strings.TrimSpace(companyInfo) == "" {
		return nil
	}
	counts := map[string]int{}
	for _, m := range techTermPattern.FindAllString(companyInfo, -1) {
		if commonEnglishWords[strings.ToLower(m)] {
			continue
		}
		counts[m]++
	}
	terms := make([]string, 0, len(counts))
	for t := range counts {
		terms = append(terms, t)
	}
	// 出現回数が多い語を優先。同数なら表記順で安定させる
	// （順序が実行ごとに変わると、認識結果の再現性が落ちる）
	sort.Slice(terms, func(i, j int) bool {
		if counts[terms[i]] != counts[terms[j]] {
			return counts[terms[i]] > counts[terms[j]]
		}
		return terms[i] < terms[j]
	})
	if len(terms) > MaxTechTerms {
		terms = terms[:MaxTechTerms]
	}
	return terms
}
