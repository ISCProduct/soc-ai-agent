package company

// 企業の公式サイトを直接読んで本文テキストを集める。
//
// 企業情報の取得は web_search(OpenAI Responses API)に頼ってきた。検索結果が
// 固定8,000トークン/call として課金されるため、企業検索まわりのコストの大半を
// 占めている(#1124)。公式サイトのURLが分かっているなら、そのページからも
// 同じ事実が取れることがある。
//
// ただし web_search の置き換えにはならない。3社で実測した充足率は次のとおりで、
// web_search のほうが安定して埋まる。
//
//	メルカリ    サイト 7/9  web_search 9/9
//	freee      サイト 8/9  web_search 8/9
//	サイボウズ  サイト 4/9  web_search 9/9
//
// 特に「勤務スタイル」は会社概要ページに書かれている情報ではないため、ほぼ取れない。
// これが enrichGapsWithAI の穴判定に含まれている限り、結局 web_search は呼ばれる。
// 現状は「たまたま全部埋まれば Search を省ける」程度の位置づけで、情報量を
// 減らさないことを優先している。
//
// JavaScript で本文を描画するサイトでは文字がほとんど取れない(トヨタで648字)。
// 薄ければ呼び出し側が web_search へ戻す。

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// minWebsiteTextRunes はこれを下回ったら「本文が取れなかった」と判断する閾値。
	// 実測ではJS描画中心のサイトが数百字、静的なコーポレートサイトは数千字になる。
	minWebsiteTextRunes = 1500

	// maxWebsiteTextRunes は LLM へ渡すテキストの上限。
	// 長さに比例して入力トークン課金が増えるので、会社概要が載る範囲で足切りする。
	maxWebsiteTextRunes = 20000

	// maxAboutPages はトップに加えて読む下位ページ数。
	// 「会社概要」は1ページに収まることが多く、増やすほど相手サーバへの負荷も増える。
	maxAboutPages = 2

	// politeDelay は同一サイトへの連続アクセスの間隔。
	politeDelay = 700 * time.Millisecond
)

var anchorRe = regexp.MustCompile(`(?is)<a[^>]+href=["']([^"']+)["'][^>]*>(.*?)</a>`)

// aboutExactLabels はリンク文字列に現れる会社概要ページの呼び名。
// ラベルの一致が最も確度の高い手がかりになる。
var aboutExactLabels = []string{
	"会社概要", "企業情報", "会社情報", "会社案内", "企業概要", "会社紹介", "コーポレート",
}

// aboutPathHints は URL 側の手がかり。ラベルが画像等で取れない場合に効く。
var aboutPathHints = []string{"/company", "/corporate", "/about", "/profile", "/overview"}

// collectWebsiteText は公式サイトのトップと会社概要ページを読み、本文を連結して返す。
// 取れた文字数が minWebsiteTextRunes 未満ならエラーを返す(呼び出し側は web_search へ戻す)。
func collectWebsiteText(ctx context.Context, websiteURL string) (string, error) {
	top := strings.TrimSpace(websiteURL)
	if top == "" {
		return "", fmt.Errorf("公式サイトURLが空です")
	}

	rawTop, err := fetchPageHTML(ctx, top)
	if err != nil {
		return "", fmt.Errorf("公式サイトの取得に失敗しました: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(normalizeHTMLText(rawTop))

	for i, link := range rankAboutLinks(rawTop, top) {
		if i >= maxAboutPages {
			break
		}
		// 相手は自社サーバではない。連続アクセスは間隔を空ける。
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(politeDelay):
		}

		rawSub, err := fetchPageHTML(ctx, link)
		if err != nil {
			continue // 1ページ落ちても残りで判断する
		}
		sb.WriteString(" ")
		sb.WriteString(normalizeHTMLText(rawSub))
	}

	text := strings.TrimSpace(sb.String())
	runes := []rune(text)
	if len(runes) < minWebsiteTextRunes {
		return "", fmt.Errorf("公式サイトから十分な本文を取得できませんでした(%d文字)", len(runes))
	}
	if len(runes) > maxWebsiteTextRunes {
		text = string(runes[:maxWebsiteTextRunes])
	}
	return text, nil
}

// fetchPageHTML は1ページ取得して文字コードをUTF-8へ揃える。
// fetchText は Content-Type の charset を見ないため、Shift-JIS のサイトが文字化けする。
func fetchPageHTML(ctx context.Context, pageURL string) (string, error) {
	type result struct {
		html string
		err  error
	}
	done := make(chan result, 1)

	go func() {
		body, charset, err := fetchBytes(pageURL)
		if err != nil {
			done <- result{err: err}
			return
		}
		decoded, err := decodeToUTF8(body, charset)
		if err != nil {
			done <- result{err: err}
			return
		}
		done <- result{html: string(decoded)}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		return r.html, r.err
	}
}

// rankAboutLinks は会社概要ページらしいリンクを確度の高い順に返す。
//
// 単純に「company を含む」で拾うと /company/books/ のような無関係なページを
// 先に読んでしまう。ラベルの一致を最優先し、階層の深さで減点する。
func rankAboutLinks(rawHTML, baseURL string) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	type candidate struct {
		url   string
		score int
	}
	seen := make(map[string]bool)
	var candidates []candidate

	for _, m := range anchorRe.FindAllStringSubmatch(rawHTML, -1) {
		href := m[1]
		label := strings.TrimSpace(normalizeHTMLText(m[2]))

		score := scoreAboutLink(href, label)
		if score == 0 {
			continue
		}

		u, err := base.Parse(href)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		// 外部サイト(採用媒体やSNS)へ出ていかない。
		if u.Host != base.Host {
			continue
		}
		u.Fragment = ""

		s := u.String()
		if seen[s] {
			continue
		}
		seen[s] = true

		// 階層が深いほど会社概要そのものから遠い(/company/ より /company/books/)。
		depth := len(strings.Split(strings.Trim(u.Path, "/"), "/"))
		candidates = append(candidates, candidate{url: s, score: score - depth*5})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.url)
	}
	return out
}

// scoreAboutLink はリンクが会社概要ページらしいかを点数にする。0 は候補外。
func scoreAboutLink(href, label string) int {
	score := 0
	for _, w := range aboutExactLabels {
		if label == w {
			score += 100
			break
		}
		if strings.Contains(label, w) {
			score += 40
			break
		}
	}
	lower := strings.ToLower(href)
	for _, w := range aboutPathHints {
		if strings.Contains(lower, w) {
			score += 20
			break
		}
	}
	return score
}
