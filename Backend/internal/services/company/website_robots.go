package company

// 公式サイトを読む前に robots.txt を確認する(RFC 9309)。
//
// 照合するのは companyfetch.BotToken 宛のグループで、無ければ * のグループ。
// 規則は最長一致で、同じ長さなら Allow を優先する。
// robots.txt が 4xx なら制限なし、5xx や通信失敗なら全面禁止とみなす。
//
// ponytail: Crawl-delay は読まない(politeDelay の 700ms 固定)。
// リダイレクト先が別ホストなら、そのホストの robots.txt は確認しない
// (同じホストへのリダイレクトは fetchPageHTML が取得前に規則と照合する)。

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"Backend/internal/companyfetch"
)

// maxRobotsBytes は読む robots.txt の上限。RFC 9309 は 500KiB 以上を求めている。
const maxRobotsBytes = 512 * 1024

type robotsRule struct {
	allow   bool
	pattern string
}

// robotsRules は自分に適用される規則。nil なら制限なし。
type robotsRules struct {
	rules []robotsRule
}

// parseRobots は robots.txt の本文から自分に適用される規則を取り出す。
func parseRobots(body string) *robotsRules {
	type group struct {
		agents []string
		rules  []robotsRule
	}
	var groups []*group
	var cur *group
	inAgents := false

	// RFC 9309 §2.2 は CR 単独の改行も認める。Scanner は LF でしか区切らないので揃える。
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), maxRobotsBytes)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "user-agent":
			// 規則の後に来た User-agent は新しいグループの始まり
			if !inAgents {
				cur = &group{}
				groups = append(groups, cur)
				inAgents = true
			}
			cur.agents = append(cur.agents, strings.ToLower(value))
		case "allow", "disallow":
			inAgents = false
			if cur == nil || value == "" {
				continue // 空の Disallow は「制限なし」
			}
			cur.rules = append(cur.rules, robotsRule{allow: key == "allow", pattern: normalizeRobotsPath(value)})
		}
	}

	collect := func(agent string) ([]robotsRule, bool) {
		var rules []robotsRule
		found := false
		for _, g := range groups {
			for _, a := range g.agents {
				if a == agent {
					rules = append(rules, g.rules...)
					found = true
					break
				}
			}
		}
		return rules, found
	}
	if rules, ok := collect(strings.ToLower(companyfetch.BotToken)); ok {
		return &robotsRules{rules: rules}
	}
	rules, _ := collect("*")
	return &robotsRules{rules: rules}
}

// allows は rawURL を取得してよいかを返す。
func (r *robotsRules) allows(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if path == "/robots.txt" || r == nil {
		return true
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	path = normalizeRobotsPath(path)

	best, allowed := -1, true
	for _, rule := range r.rules {
		if !matchRobotsPattern(rule.pattern, path) {
			continue
		}
		n := len(rule.pattern)
		if n > best || (n == best && rule.allow) {
			best, allowed = n, rule.allow
		}
	}
	return allowed
}

// normalizeRobotsPath は規則とURLを同じ表現に揃える(RFC 9309 §2.2.2)。
// 非予約文字の %XX は復号し、非ASCII は %XX(大文字)へ符号化する。
// 予約文字の %XX(%2F など)は意味が変わるので復号しない。
func normalizeRobotsPath(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			d := unhex(s[i+1])<<4 | unhex(s[i+2])
			if isUnreserved(d) {
				b.WriteByte(d)
			} else {
				b.WriteByte('%')
				b.WriteByte(hex[d>>4])
				b.WriteByte(hex[d&15])
			}
			i += 2
		case c >= 0x80:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	default:
		return c - 'a' + 10
	}
}

func isUnreserved(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

// matchRobotsPattern は * (任意の文字列) と末尾の $ (終端) を解釈して前方一致を判定する。
func matchRobotsPattern(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	expr := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*")
	if anchored {
		expr += "$"
	}
	re, err := regexp.Compile(expr)
	return err == nil && re.MatchString(path)
}

// fetchRobots は siteURL と同じホストの robots.txt を取得して解釈する。
// エラーは「確認できなかった」ことを表し、呼び出し側はそのサイトを読まないこと。
func fetchRobots(ctx context.Context, siteURL string) (*robotsRules, error) {
	u, err := url.Parse(strings.TrimSpace(siteURL))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("公式サイトURLを解釈できません: %q", siteURL)
	}
	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"

	if err := checkFetchURL(robotsURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", companyfetch.UserAgent())
	resp, err := newFetchClient(15*time.Second, nil).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxRobotsBytes))
		if err != nil {
			return nil, err
		}
		return parseRobots(string(body)), nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, nil // robots.txt が無い = 制限なし
	default:
		return nil, fmt.Errorf("robots.txt の取得に失敗しました: status %d", resp.StatusCode)
	}
}
