package companyfetch

import (
	"testing"
	"time"

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

func TestExtractJSONObject(t *testing.T) {
	obj, err := ExtractJSONObject("prefix {\"a\":1} suffix")
	assert.NoError(t, err)
	assert.Equal(t, `{"a":1}`, obj)

	_, err = ExtractJSONObject("no json")
	assert.Error(t, err)
}
