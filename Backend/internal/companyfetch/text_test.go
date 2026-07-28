package companyfetch

import (
	"context"
	"testing"
	"time"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/stretchr/testify/assert"
)

func TestIsFresh(t *testing.T) {
	now := time.Now()
	fresh := now.Add(-1 * time.Hour)
	stale := now.Add(-100 * 24 * time.Hour)

	assert.False(t, IsFresh(nil, TTLInfo))
	assert.True(t, IsFresh(&fresh, TTLInfo))
	assert.False(t, IsFresh(&stale, TTLInfo))
	assert.True(t, IsFresh(&fresh, TTLJobs))
	assert.False(t, IsFresh(&stale, TTLJobs))
}

func TestIsEmptyTechPayload(t *testing.T) {
	assert.True(t, IsEmptyTechPayload(""))
	assert.True(t, IsEmptyTechPayload("  "))
	assert.True(t, IsEmptyTechPayload("[]"))
	assert.True(t, IsEmptyTechPayload("null"))
	assert.True(t, IsEmptyTechPayload("{}"))
	assert.False(t, IsEmptyTechPayload(`["Go"]`))
	assert.False(t, IsEmptyTechPayload("Go, TypeScript"))
}

func TestHasTechData(t *testing.T) {
	assert.False(t, HasTechData("", "", "", ""))
	assert.False(t, HasTechData("[]", "", "", ""))
	assert.True(t, HasTechData(`["Go"]`, "", "", ""))
	assert.True(t, HasTechData("", `["AWS"]`, "", ""))
	assert.True(t, HasTechData("", "", `["Actions"]`, ""))
	assert.True(t, HasTechData("", "", "", "スクラム"))
}

func TestHasMeaningfulMarketInfo(t *testing.T) {
	assert.False(t, HasMeaningfulMarketInfo(false, "", ""))
	assert.False(t, HasMeaningfulMarketInfo(false, "unlisted", ""))
	assert.False(t, HasMeaningfulMarketInfo(false, "UNLISTED", "  "))
	assert.True(t, HasMeaningfulMarketInfo(true, "unlisted", ""))
	assert.True(t, HasMeaningfulMarketInfo(false, "prime", ""))
	assert.True(t, HasMeaningfulMarketInfo(false, "standard", ""))
	assert.True(t, HasMeaningfulMarketInfo(false, "growth", ""))
	assert.True(t, HasMeaningfulMarketInfo(false, "unlisted", "4755"))
}

func TestTrimText(t *testing.T) {
	assert.Equal(t, "abc", TrimText("  abc  ", 10))
	long := "あいうえおかきくけこ"
	assert.Equal(t, "あいうえお", TrimText(long, 5))
}

func TestNormalizeHTMLText(t *testing.T) {
	html := `<html><script>x()</script><body><p>Hello&nbsp;World</p><style>.a{}</style></body></html>`
	got := NormalizeHTMLText(html)
	assert.Contains(t, got, "Hello World")
	assert.NotContains(t, got, "script")
	assert.NotContains(t, got, ".a{}")
}

func TestCandidateCareerURLs(t *testing.T) {
	assert.Nil(t, CandidateCareerURLs(""))
	urls := CandidateCareerURLs("https://example.com/")
	assert.Equal(t, "https://example.com", urls[0])
	assert.Contains(t, urls, "https://example.com/careers")
	assert.Contains(t, urls, "https://example.com/recruit")
}

func TestValidatePublicHTTPURL(t *testing.T) {
	assert.NoError(t, ValidatePublicHTTPURL("https://example.com/careers"))
	assert.Error(t, ValidatePublicHTTPURL("file:///etc/passwd"))
	assert.Error(t, ValidatePublicHTTPURL("http://127.0.0.1/"))
	assert.Error(t, ValidatePublicHTTPURL("http://10.0.0.1/"))
	assert.Error(t, ValidatePublicHTTPURL("http://localhost/"))
}

func TestFirstFetchableText_Success(t *testing.T) {
	// Acquire は gBiz→Search に移行済みだが、スクレイプユーティリティ自体の成功パスを担保する
	allowPrivateURLs = true
	t.Cleanup(func() { allowPrivateURLs = false })

	body := strings.Repeat("企業の採用情報です。", 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><p>" + body + "</p></body></html>"))
	}))
	t.Cleanup(srv.Close)

	text, sourceURL, err := FirstFetchableText(context.Background(), []string{srv.URL + "/careers"})
	assert.NoError(t, err)
	assert.Equal(t, srv.URL+"/careers", sourceURL)
	assert.Contains(t, text, "企業の採用情報")
}

func TestFirstFetchableText_RejectsPrivateURL(t *testing.T) {
	_, _, err := FirstFetchableText(context.Background(), []string{"http://127.0.0.1/secret"})
	assert.Error(t, err)
}

func TestExtractJSONObject(t *testing.T) {
	obj, err := ExtractJSONObject("prefix {\"a\":1} suffix")
	assert.NoError(t, err)
	assert.Equal(t, `{"a":1}`, obj)

	_, err = ExtractJSONObject("no json")
	assert.Error(t, err)
}
