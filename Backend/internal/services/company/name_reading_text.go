package company

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/unicode/norm"
)

var (
	spaceRunRe     = regexp.MustCompile(`[\s\x{00a0}\x{3000}]+`)
	readingLabelRe = regexp.MustCompile(`フリガナ|ふりがな|ふり仮名|読み方|よみがな|ヨミガナ|ヨミ|Furigana`)
	legalFormWords = []string{"株式会社", "有限会社", "合同会社", "合資会社", "合名会社", "(株)", "（株）", "㈱", "(有)", "（有）"}
	legalFormKana  = []string{"カブシキガイシャ", "ユウゲンガイシャ", "ゴウドウガイシャ", "ゴウシガイシャ", "ゴウメイガイシャ"}
)

// nameCore は、企業名から法人格を除いた部分（ページ内で社名を探すのに使う）。
func nameCore(name string) string {
	s := strings.TrimSpace(name)
	for _, w := range legalFormWords {
		s = strings.ReplaceAll(s, w, "")
	}
	return strings.TrimSpace(s)
}

// stripLegalFormReading は、読みの先頭・末尾の法人格の読み（カブシキガイシャ等）を除く。
// 法人番号公表サイトのフリガナは、法人格を含まないので、そろえる。
func stripLegalFormReading(reading string) string {
	r := reading
	for _, k := range legalFormKana {
		r = strings.TrimPrefix(r, k)
		r = strings.TrimSuffix(r, k)
	}
	return strings.Trim(r, " 　・")
}

// squashSpaces は、空白の連続を 1 つの半角空白にして、前後を除く（引用がページに含まれるかの比較用）。
func squashSpaces(s string) string { return strings.TrimSpace(spaceRunRe.ReplaceAllString(s, " ")) }

// kanaKey は、読みの比較用に、全角・半角、ひらがな・カタカナ、空白・中黒・括弧をそろえる。
func kanaKey(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKC.String(s) {
		switch {
		case r >= 'ぁ' && r <= 'ゖ':
			b.WriteRune(r + 0x60)
		case r == ' ' || r == '・' || r == '（' || r == '）' || r == '(' || r == ')' || r == '　':
			// 比較から除く
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pageTextWithRuby は HTML を文章にする。<ruby>漢字<rt>よみ</rt></ruby> は「漢字（よみ）」の形で残す
// （通常のテキスト化では、読みの情報が消える、または漢字と読みが連なってしまう）。
func pageTextWithRuby(html string) string { return pageText(html, true) }

// pageTextPlain は、ルビの読みを除いた文章（社名が、ルビで分断されずに続く形）。社名を探す位置の特定に使う。
func pageTextPlain(html string) string { return pageText(html, false) }

func pageText(html string, keepRuby bool) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	doc.Find("script,style,noscript").Remove()
	doc.Find("ruby").Each(func(_ int, s *goquery.Selection) {
		rt := strings.TrimSpace(s.Find("rt").Text())
		s.Find("rt,rp").Remove()
		base := strings.TrimSpace(s.Text())
		if rt != "" && keepRuby {
			s.ReplaceWithHtml(base + "（" + rt + "）")
		} else {
			s.ReplaceWithHtml(base)
		}
	})
	// ブロックの境界で改行を入れ、表の項目名と値が連なるのを避ける
	doc.Find("th,td,li,p,h1,h2,h3,h4,dt,dd,div,br,tr").Each(func(_ int, s *goquery.Selection) { s.AppendHtml(" ") })
	return squashSpaces(doc.Find("body").Text())
}

// evidenceWindows は、ページ内の「企業名の前後」と「フリガナ等の語の前後」を取り出して、AI に渡す文章にする。
// ページ全体を渡さないのは、費用と、無関係な文章に含まれる指示文の影響を減らすため。社名の記載が無ければ空。
func evidenceWindows(text, plain, core string) string {
	// 社名は、ルビを除いた文章（plain）で探す。ルビ入りの文章では「山田（やまだ）製作所」のように分断されるため。
	// 見つけた位置は、plain の先頭からの割合で、ルビ入りの文章（text）の位置へ写す（ルビの分だけ文章が長い）。
	runes := []rune(text)
	plainRunes := []rune(plain)
	type span struct{ a, b int }
	var spans []span
	add := func(idx, radius int) {
		a, b := idx-radius, idx+radius
		if a < 0 {
			a = 0
		}
		if b > len(runes) {
			b = len(runes)
		}
		if n := len(spans); n > 0 && a <= spans[n-1].b {
			if b > spans[n-1].b {
				spans[n-1].b = b
			}
			return
		}
		spans = append(spans, span{a, b})
	}
	coreRunes := []rune(core)
	found := false
	for i := 0; i+len(coreRunes) <= len(plainRunes) && len(spans) < 8; i++ {
		if string(plainRunes[i:i+len(coreRunes)]) == core {
			found = true
			pos := i
			if len(plainRunes) > 0 {
				pos = int(float64(i) / float64(len(plainRunes)) * float64(len(runes)))
			}
			add(pos, windowRadius+len(coreRunes))
			i += len(coreRunes) - 1
		}
	}
	if !found {
		return ""
	}
	for _, loc := range readingLabelRe.FindAllStringIndex(text, 4) {
		add(utf8.RuneCountInString(text[:loc[0]]), 60)
	}
	var parts []string
	total := 0
	for _, sp := range spans {
		seg := string(runes[sp.a:sp.b])
		if total+utf8.RuneCountInString(seg) > maxEvidenceRunes {
			break
		}
		parts = append(parts, seg)
		total += utf8.RuneCountInString(seg)
	}
	return strings.Join(parts, "\n---\n")
}
