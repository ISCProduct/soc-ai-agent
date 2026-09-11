package company

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// fetchBytes URLからレスポンスのバイト列とContent-TypeのCharsetを返す
func fetchBytes(url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	client := &http.Client{Timeout: 30 * time.Second}
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0; +https://example.com/bot)")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func normalizeHTMLText(rawHTML string) string {
	clean := regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</\1>`).ReplaceAllString(rawHTML, " ")
	clean = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(clean, " ")
	clean = html.UnescapeString(clean)
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	clean = regexp.MustCompile(`\s+`).ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

func ExtractCharset(contentType string) string { return extractCharset(contentType) }
