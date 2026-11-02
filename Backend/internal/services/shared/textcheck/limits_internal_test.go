package textcheck

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// RAG（rag/services/es_review.py）の字数の定義と食い違っていないことを固定する。
// RAG 側だけを変えると、改善文の「収まっています」と原文の「字数超過」が
// 同じ画面で食い違う。ここが落ちたら CharLimit / CountChars も合わせること。
func TestCharLimit_MatchesRAGDefinition(t *testing.T) {
	src, err := os.ReadFile("../../../../../rag/services/es_review.py")
	if err != nil {
		t.Skipf("RAG のソースが読めないため省略: %v", err)
	}
	s := string(src)

	const wantCount = `return len(text.replace("\r", "").replace("\n", "").strip())`
	if !strings.Contains(s, wantCount) {
		t.Errorf("count_es_chars の数え方が変わった可能性がある。CountChars を見直すこと（想定: %s）", wantCount)
	}

	re := regexp.MustCompile(`"(within|around)":\s*\(([0-9.]+),\s*([0-9.]+)\)`)
	found := 0
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		low, _ := strconv.ParseFloat(m[2], 64)
		high, _ := strconv.ParseFloat(m[3], 64)
		if got := charLimitRange[m[1]]; got != [2]float64{low, high} {
			t.Errorf("%s の許容範囲: Go %v / RAG (%v, %v)", m[1], got, low, high)
		}
		found++
	}
	if found != len(charLimitRange) {
		t.Errorf("RAG の _CHAR_LIMIT_RANGE から %d 件しか読めない（Go は %d 件）", found, len(charLimitRange))
	}
}
