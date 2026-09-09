// Package sttbench は音声認識モデルの品質比較に使う評価ロジック。
//
// ネットワークに触れない純粋関数として分けてある。
// APIキーが無い環境でも指標の正しさを単体テストで確認できるようにするため。
package sttbench

import (
	"regexp"
	"strings"
	"unicode"
)

// NormalizeForCER は文字誤り率の比較用に表記を揃える。
//
// 句読点・空白・記号は落とす。面接の評価に効くのは語の中身であって
// 読点の位置ではなく、そこを数えると本当の誤りが薄まる。
func NormalizeForCER(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			continue
		case strings.ContainsRune("、。，．,.!?！？「」『』（）()・…‥ー-", r):
			// 長音・ハイフンまで落とすのは、表記ゆれ（サーバ/サーバー）を
			// 誤りとして数えないため
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// CER は文字誤り率（0.0〜）。編集距離 / 正解文字数。
// 正解が空なら、認識が空のとき0、それ以外は1を返す。
func CER(reference, hypothesis string) float64 {
	ref := []rune(NormalizeForCER(reference))
	hyp := []rune(NormalizeForCER(hypothesis))
	if len(ref) == 0 {
		if len(hyp) == 0 {
			return 0
		}
		return 1
	}
	return float64(levenshtein(ref, hyp)) / float64(len(ref))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// numberPattern は数値・割合・年数など、誤ると意味が変わる表現。
// 全角数字も拾う。
var numberPattern = regexp.MustCompile(`[0-9０-９]+(?:[.．][0-9０-９]+)?`)

// ExtractNumbers は文中の数値表現を出現順に返す。
//
// 面接では「120時間」「30パーセント」を取り違えると成果の意味が変わる。
// 一般的なCERでは1〜2文字の差にしかならず埋もれるため、別指標にする。
func ExtractNumbers(s string) []string {
	raw := numberPattern.FindAllString(s, -1)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		out = append(out, normalizeDigits(r))
	}
	return out
}

func normalizeDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '０' && r <= '９':
			b.WriteRune(r - '０' + '0')
		case r == '．':
			b.WriteRune('.')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NumberAccuracy は正解に含まれる数値が認識結果にも同じ多重度で現れた割合。
//
// 出現順ではなく多重集合で比較する。語順が入れ替わっても
// 数値そのものが保たれていれば意味は壊れない。
func NumberAccuracy(reference, hypothesis string) (matched, total int) {
	refNums := ExtractNumbers(reference)
	hypCount := map[string]int{}
	for _, n := range ExtractNumbers(hypothesis) {
		hypCount[n]++
	}
	for _, n := range refNums {
		total++
		if hypCount[n] > 0 {
			hypCount[n]--
			matched++
		}
	}
	return matched, total
}

// CheckableKeywords は manifest の checks から、実際に照合できる語だけを残す。
//
// checks には「Go」「MySQL」のような実際に発話へ現れる語と、
// 「一般語」「言い淀み」「無音区間」のような評価観点のラベルが混在している。
// ラベルまで照合すると、認識結果に現れるはずのない語を不正解として数え、
// 固有名詞の正解率を実態より大幅に低く見せる（実測で60%まで下がった）。
//
// 正解文に含まれる語だけを照合対象とすることで、ラベルを自動的に除外する。
func CheckableKeywords(keywords []string, reference string) []string {
	low := strings.ToLower(reference)
	out := make([]string, 0, len(keywords))
	for _, k := range keywords {
		if strings.Contains(low, strings.ToLower(k)) {
			out = append(out, k)
		}
	}
	return out
}

// KeywordHits は固有名詞・技術用語が認識結果に含まれるかを判定する。
//
// 大文字小文字は無視する。「aws」と「AWS」は面接の理解として同義。
// keywords は CheckableKeywords を通した後の語を渡すこと。
func KeywordHits(keywords []string, hypothesis string) (hit []string, miss []string) {
	low := strings.ToLower(hypothesis)
	for _, k := range keywords {
		if strings.Contains(low, strings.ToLower(k)) {
			hit = append(hit, k)
		} else {
			miss = append(miss, k)
		}
	}
	return hit, miss
}

// IsRecognitionFailure は「認識できなかった」とみなす結果か。
//
// 空文字だけでなく、音声長に対して極端に短い結果も失敗として扱う。
// 数文字だけ返る場合、後段のLLMは無言と区別できず会話が破綻する。
//
// minCharsPerSec は「1秒あたり最低これだけの文字が出るはず」という下限。
func IsRecognitionFailure(text string, durationSec, minCharsPerSec float64) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if durationSec <= 0 || minCharsPerSec <= 0 {
		return false
	}
	return float64(len([]rune(t))) < durationSec*minCharsPerSec
}

// orthographyVariants は「意味は同じだが表記が違う」対。
//
// STTは読み上げ内容を正しく捉えていても表記を選び直す。
// これを誤変換として数えると、実際には問題ない差でCERが跳ね上がり、
// モデル比較の判断を誤らせる（実測で 30パーセント→30% だけでCER 0.08）。
//
// 意味が変わる置換は入れないこと。ここに足すたびに
// 「本当に同義か」を確認する必要がある。
var orthographyVariants = [][2]string{
	{"パーセント", "%"},
	{"ウェブ", "web"},
	{"ヴ", "ブ"},
}

// NormalizeOrthography は表記ゆれを吸収した比較用の文字列を返す。
// NormalizeForCER の後段に置く想定。
func NormalizeOrthography(s string) string {
	out := strings.ToLower(s)
	for _, v := range orthographyVariants {
		out = strings.ReplaceAll(out, strings.ToLower(v[0]), strings.ToLower(v[1]))
	}
	return out
}

// SemanticCER は表記ゆれを除いた文字誤り率。
//
// CER は「文字がどれだけ違うか」、SemanticCER は「意味がどれだけ違うか」の近似。
// モデルの採否は後者で判断する。前者だけ見ると、
// 表記の癖が違うだけのモデルを不当に低く評価する。
// 表記ゆれの吸収を先に行う。NormalizeForCER は記号を落とすため、
// 先に通すと「%」が消えて「パーセント」との対応が取れなくなる。
func SemanticCER(reference, hypothesis string) float64 {
	ref := []rune(NormalizeForCER(NormalizeOrthography(reference)))
	hyp := []rune(NormalizeForCER(NormalizeOrthography(hypothesis)))
	if len(ref) == 0 {
		if len(hyp) == 0 {
			return 0
		}
		return 1
	}
	return float64(levenshtein(ref, hyp)) / float64(len(ref))
}
