package companyfetch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RobotsAllowed は pageURL を取得してよいかを、そのサイトの robots.txt で判定する（SOCAIAGENT-723）。
//
// 判定は RFC 9309 に沿う。
//   - robots.txt が 4xx（無い）なら全て許可。
//   - 5xx・接続できないなら全て不許可（相手が落ちているときに取りに行かない）。
//   - 「User-agent: *」のグループだけを見る（自前の User-Agent 名で個別に書くサイトは想定しない）。
//   - Allow / Disallow は最も長く一致した規則に従い、同じ長さなら Allow を優先する。
//
// ワイルドカード（* と $）は解釈しない（簡略化）。使うサイトが増えたら対応する。
func RobotsAllowed(ctx context.Context, pageURL string) (bool, error) {
	u, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil || u.Host == "" {
		return false, fmt.Errorf("invalid url: %q", pageURL)
	}
	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	if err := ValidatePublicHTTPURL(robotsURL); err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := NewSafeHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 500:
		return false, nil
	case resp.StatusCode >= 400:
		return true, nil
	case resp.StatusCode >= 300:
		// リダイレクトの上限を超えた場合など。安全側に倒す。
		return false, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return false, nil
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return robotsPathAllowed(string(body), path), nil
}

// robotsPathAllowed は robots.txt の本文から「User-agent: *」の規則を読み、path を許可するかを返す。
func robotsPathAllowed(robots, path string) bool {
	inStar := false    // 今のグループが * を含むか
	lastWasUA := false // 直前の行が User-agent か（連続する User-agent は同じグループ）
	bestLen, allowed := -1, true
	sc := bufio.NewScanner(strings.NewReader(robots))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent":
			if !lastWasUA {
				inStar = false
			}
			if val == "*" {
				inStar = true
			}
			lastWasUA = true
		case "allow", "disallow":
			lastWasUA = false
			if !inStar || val == "" || !strings.HasPrefix(path, val) {
				continue
			}
			isAllow := key == "allow"
			if len(val) > bestLen || (len(val) == bestLen && isAllow) {
				bestLen, allowed = len(val), isAllow
			}
		default:
			lastWasUA = false
		}
	}
	return allowed
}
