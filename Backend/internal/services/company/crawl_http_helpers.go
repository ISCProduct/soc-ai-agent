package company

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"Backend/internal/companyfetch"
	"Backend/internal/netsafe"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// 外部サイト（公式サイト・クロールソース）の取得は、すべてここの client を通す(#2489)。
// 内部アドレス（ループバック・プライベート・リンクローカル。クラウドのメタデータを含む）は
// 取りに行かない。検査は2段で、どちらも companyfetch / name_reading_site と同じもの。
//
//	checkFetchURL  : 取得前とリダイレクトの各段で、スキームとホスト表記を見る
//	fetchTransport : 接続直前に、実際につなぐIPを見る（DNSの差し替え対策。netsafe）

// testAllowedAddrs は単体テスト専用。httptest のサーバー（"127.0.0.1:port"）だけを
// 検査の対象外にする。本番では常に空で、すべての取得が検査を通る。
var testAllowedAddrs []string

// fetchTransport は外部サイト取得で共有する Transport。
// 呼び出しごとに Transport を作ると、使い終わった接続がプールに残り続ける。
var fetchTransport = &http.Transport{
	DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		if slices.Contains(testAllowedAddrs, addr) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
		}
		return netsafe.DialContext(ctx, network, addr)
	},
}

// checkFetchURL は取得してよい URL かを見る。拒否したらホスト名だけをログに出す。
func checkFetchURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && slices.Contains(testAllowedAddrs, u.Host) {
		return nil
	}
	if err := companyfetch.ValidatePublicHTTPURL(rawURL); err != nil {
		host := ""
		if u != nil {
			host = u.Hostname()
		}
		log.Printf("[fetch] 取得を拒否しました host=%q: %v", host, err)
		return err
	}
	return nil
}

// newFetchClient は外部サイト取得用の client を返す。
// onRedirect はリダイレクトごとの追加の検査（nil 可。robots.txt の照合など）。
func newFetchClient(timeout time.Duration, onRedirect func(*http.Request) error) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: fetchTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("リダイレクトが多すぎます")
			}
			if err := checkFetchURL(req.URL.String()); err != nil {
				return err
			}
			if onRedirect != nil {
				return onRedirect(req)
			}
			return nil
		},
	}
}

// fetchBytes URLからレスポンスのバイト列とContent-TypeのCharsetを返す
func fetchBytes(url string) ([]byte, string, error) {
	return fetchBytesWith(newFetchClient(30*time.Second, nil), url)
}

// fetchBytesWith は fetchBytes と同じだが、リダイレクトの扱いなどを client で指定できる。
// client は newFetchClient で作ったものを渡すこと。
func fetchBytesWith(client *http.Client, url string) ([]byte, string, error) {
	if err := checkFetchURL(url); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", companyfetch.UserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	charset := extractCharset(ct)
	return body, charset, nil
}

// extractCharset Content-TypeヘッダーからCharsetを抽出する
func extractCharset(contentType string) string {
	lower := strings.ToLower(contentType)
	if strings.Contains(lower, "shift_jis") || strings.Contains(lower, "shift-jis") || strings.Contains(lower, "sjis") {
		return "shift_jis"
	}
	if strings.Contains(lower, "euc-jp") {
		return "euc-jp"
	}
	return "utf-8"
}

// decodeToUTF8 Shift-JIS/EUC-JPをUTF-8に変換する
func decodeToUTF8(b []byte, charset string) ([]byte, error) {
	switch charset {
	case "shift_jis":
		decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), b)
		return decoded, err
	case "euc-jp":
		decoded, _, err := transform.Bytes(japanese.EUCJP.NewDecoder(), b)
		return decoded, err
	default:
		return b, nil
	}
}

func fetchText(url string) (string, error) {
	body, _, err := fetchBytes(url)
	return string(body), err
}

// Goの正規表現(RE2)は後方参照を持たないため、`<(script|style)...</\1>` は
// コンパイル時に panic する。script と style を交替で個別に書く。
// あわせてコンパイルを1回で済ませる(呼ばれるたびに3本コンパイルしていた)。
var (
	scriptStyleBlockRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	htmlTagRe          = regexp.MustCompile(`(?is)<[^>]+>`)
	whitespaceRunRe    = regexp.MustCompile(`\s+`)
)

func normalizeHTMLText(rawHTML string) string {
	clean := scriptStyleBlockRe.ReplaceAllString(rawHTML, " ")
	clean = htmlTagRe.ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	clean = whitespaceRunRe.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

func ExtractCharset(contentType string) string { return extractCharset(contentType) }
