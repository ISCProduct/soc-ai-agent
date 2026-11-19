package interview

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/width"
)

// 根拠照合の補助（SOCAIAGENT-675）。漢数字の数値化と、カタカナ・英字の語（固有名詞の候補）の取り出し。
//
// 内容語の局所類似度（ValidateEvidence）では、発話の語彙を流用して事実だけ入れ替えた捏造
// （「Javaで作った」→「Pythonで作った」、「三十人」→「四十人」）を正当な要約と分けられない。
// 数値と、カタカナ・英字の語は完全一致で別に照合する。LLM は呼ばない。

// kanjiNumberPattern は漢数字の並びと、続く助数詞。
//
// 数値にするのは次の2つだけ（U5）。それ以外の漢数字は語の一部として扱う。
//   - 助数詞が続く（三倍・二週間・百件）
//   - 位取り（十・百・千・万・億）を含む2文字以上の並び（五十・二百）
//
// 「一緒」「一番」「一つ」「一度」は助数詞に入れない（数値ではない言い回しの方が多い）。
// 「十分」は「じゅうぶん」が多いので数値にしない（kanjiNumberExcluded）。
// 「一時」は「一時的」が多いので、時は「時間」だけを助数詞にする。
var kanjiNumberPattern = regexp.MustCompile(
	`([〇零一二三四五六七八九十百千万億]+)(パーセント|種類|週間|時間|分間|年間|日間|か月|ヶ月|ケ月|カ月|箇月|キロ|人|名|倍|円|年|月|週|日|秒|回|件|個|社|割|位|点|冊|本|枚|歳|才|校|店|台|種|%|％)?`,
)

// kanjiNumberExcluded は、助数詞が続いても数値にしない並び（読みが数値でないことが多いもの）。
var kanjiNumberExcluded = map[string]bool{"十分": true}

var kanjiDigits = map[rune]int64{'〇': 0, '零': 0, '一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
var kanjiSmallUnits = map[rune]int64{'十': 10, '百': 100, '千': 1000}
var kanjiLargeUnits = map[rune]int64{'万': 10_000, '億': 100_000_000}

// extractKanjiNumbers は漢数字の数値を算用数字の文字列で返す（三倍→"3"、五十人→"50"）。
func extractKanjiNumbers(s string) []string {
	var out []string
	for _, m := range kanjiNumberPattern.FindAllStringSubmatch(s, -1) {
		digits, counter := m[1], m[2]
		if kanjiNumberExcluded[digits+counter] {
			continue
		}
		if counter == "" && !(utf8RuneCount(digits) >= 2 && strings.ContainsAny(digits, "十百千万億")) {
			continue
		}
		if v, ok := parseKanjiNumber(digits); ok {
			out = append(out, strconv.FormatInt(v, 10))
		}
	}
	return out
}

func utf8RuneCount(s string) int { return len([]rune(s)) }

// parseKanjiNumber は位取りの漢数字を数値にする（二百五十→250、三万二千→32000、二〇二五→2025）。
// 位取りの無い並び（二〇二五）は桁を並べたものとして読む。読めない並びは ok=false。
func parseKanjiNumber(s string) (int64, bool) {
	runes := []rune(s)
	if !strings.ContainsAny(s, "十百千万億") {
		var v int64
		for _, r := range runes {
			d, ok := kanjiDigits[r]
			if !ok {
				return 0, false
			}
			v = v*10 + d
		}
		return v, true
	}
	var total, section, current int64
	seenDigit := false
	for _, r := range runes {
		switch {
		case kanjiDigits[r] > 0 || r == '〇' || r == '零':
			current = current*10 + kanjiDigits[r]
			seenDigit = true
		case kanjiSmallUnits[r] > 0:
			if current == 0 {
				current = 1
			}
			section += current * kanjiSmallUnits[r]
			current = 0
			seenDigit = true
		case kanjiLargeUnits[r] > 0:
			if section+current == 0 {
				return 0, false
			}
			total += (section + current) * kanjiLargeUnits[r]
			section, current = 0, 0
		default:
			return 0, false
		}
	}
	if !seenDigit && total == 0 {
		return 0, false
	}
	return total + section + current, true
}

// properNounPattern はカタカナの語（2文字以上）と英字の語（2文字以上）。
// カタカナ語は固有名詞に限らず全部を対象にする（U6）。除外リストの要否は誤検知率を見て決める。
var properNounPattern = regexp.MustCompile(`[ァ-ヺー]{2,}|[a-z][a-z0-9+#]+`)

// normalizeForTerms は語の照合用に幅と大文字小文字をそろえる（ＧｉｔＨｕｂ→github、ｶﾌｪ→カフェ）。
func normalizeForTerms(s string) string {
	return strings.ToLower(width.Fold.String(s))
}

// extractTerms はカタカナ・英字の語を、正規化して重複なしで返す（並びは安定させる）。
// 長音だけの並び（ーー）は語にしない。
func extractTerms(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range properNounPattern.FindAllString(normalizeForTerms(s), -1) {
		if strings.Trim(term, "ー") == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}

// quotedPattern は「」で囲んだ引用。
var quotedPattern = regexp.MustCompile(`「([^「」]*)」`)

// termSource は語を照合する範囲を返す。「」の引用があれば引用の中だけ、無ければ全文。
//
// 教員向けの根拠は「引用＋指導ポイント」で、引用の外は講評（「他メンバーとの分担に触れていない」）。
// 講評の語は受験者が言ったという主張ではないので照合しない。学生向けの根拠は引用そのものなので全文を見る。
func termSource(text string) string {
	matches := quotedPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text
	}
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, m[1])
	}
	return strings.Join(parts, "\n")
}

// hasUnspokenTerm は evidence に、受験者の発話に出てこないカタカナ・英字の語があるかを返す。
// 発話側は正規化した全文に含まれるかで見る（語の切れ目の違い「エクセルマクロ」を許すため）。
func hasUnspokenTerm(text string, spoken SpokenEvidence) bool {
	for _, term := range extractTerms(termSource(text)) {
		if !strings.Contains(spoken.normalized, term) {
			return true
		}
	}
	return false
}
