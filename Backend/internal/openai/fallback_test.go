package openai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// stubGuard はテスト用の FallbackGuard。
type stubGuard struct {
	allow  bool
	reason string
	calls  int32
}

func (g *stubGuard) AllowFallback() (bool, string) {
	atomic.AddInt32(&g.calls, 1)
	if g.allow {
		return true, ""
	}
	return false, g.reason
}

// localAndFallbackServers はローカル推論先とフォールバック先のモックを1組返す。
// localStatus が 0 のときはローカル側を即閉じて接続エラーを再現する。
func localAndFallbackServers(t *testing.T, localStatus int) (localURL, fallbackURL string, localHits, fbHits *int32, fbAuth *string) {
	t.Helper()
	var lh, fh int32
	auth := ""

	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&lh, 1)
		w.WriteHeader(localStatus)
		_, _ = w.Write([]byte(`{"error":{"message":"local failure"}}`))
	}))
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fh, 1)
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"from\":\"fallback\"}"}}]}`))
	}))
	t.Cleanup(func() {
		local.Close()
		fallback.Close()
	})
	if localStatus == 0 {
		local.Close() // 接続エラーを再現する
	}
	return local.URL, fallback.URL, &lh, &fh, &auth
}

// TestFallback_OnLocalServerError はローカルが 5xx のとき OpenAI へ再試行することを検証する（#1293）。
func TestFallback_OnLocalServerError(t *testing.T) {
	localURL, fbURL, localHits, fbHits, fbAuth := localAndFallbackServers(t, http.StatusInternalServerError)

	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", localURL)
	t.Setenv("AI_FALLBACK_BASE_URL", fbURL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	guard := &stubGuard{allow: true}
	cli.SetFallbackGuard(guard)

	out, err := cli.ChatCompletionJSON(context.Background(), "sys", "user", 0, 32)
	if err != nil {
		t.Fatalf("フォールバックが機能していない: %v", err)
	}
	if !strings.Contains(out, "fallback") {
		t.Errorf("フォールバック先の応答が返っていない: %q", out)
	}
	if atomic.LoadInt32(localHits) == 0 {
		t.Error("ローカルを先に試していない")
	}
	if atomic.LoadInt32(fbHits) != 1 {
		t.Errorf("フォールバック先へのリクエスト=%d want 1", atomic.LoadInt32(fbHits))
	}
	// フォールバック先には実キーを送る（ローカルにはダミーを送る）
	if *fbAuth != "Bearer sk-real" {
		t.Errorf("フォールバック先の Authorization = %q, want Bearer sk-real", *fbAuth)
	}
	if guard.calls == 0 {
		t.Error("ガードが参照されていない")
	}
}

// TestFallback_OnConnectionError はローカルに繋がらない場合も再試行することを検証する。
func TestFallback_OnConnectionError(t *testing.T) {
	localURL, fbURL, _, fbHits, _ := localAndFallbackServers(t, 0)

	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", localURL)
	t.Setenv("AI_FALLBACK_BASE_URL", fbURL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	cli.SetFallbackGuard(&stubGuard{allow: true})

	if _, err := cli.ChatCompletionJSON(context.Background(), "sys", "user", 0, 32); err != nil {
		t.Fatalf("接続エラー時にフォールバックしていない: %v", err)
	}
	if atomic.LoadInt32(fbHits) != 1 {
		t.Errorf("フォールバック先へのリクエスト=%d want 1", atomic.LoadInt32(fbHits))
	}
}

// roundTripLocal はトランスポートを直接叩く。
// 公開メソッド経由だと失敗時に指数バックオフ（2+4+8+16秒）が入り、
// 失敗経路のテストが分単位になるため。
func roundTripLocal(t *testing.T, tr *fallbackTransport, localURL string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		localURL+"/chat/completions", strings.NewReader(`{"model":"m"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+localPlaceholderAPIKey)
	return tr.RoundTrip(req)
}

// TestFallback_NotOnClientError は 4xx では再試行しないことを検証する。
// リクエスト自体の問題は OpenAI でも同じく失敗するため、課金を増やすだけになる。
func TestFallback_NotOnClientError(t *testing.T) {
	localURL, fbURL, _, fbHits, _ := localAndFallbackServers(t, http.StatusBadRequest)

	guard := &stubGuard{allow: true}
	tr := &fallbackTransport{
		primaryBaseURL: localURL, fallbackBaseURL: fbURL,
		fallbackKey: "sk-real", guard: guard, system: "text",
	}
	resp, err := roundTripLocal(t, tr, localURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400（ローカルの応答がそのまま返るべき）", resp.StatusCode)
	}
	if n := atomic.LoadInt32(fbHits); n != 0 {
		t.Errorf("4xx でフォールバックした（%d 回）", n)
	}
	if guard.calls != 0 {
		t.Error("4xx でガードを参照している")
	}
}

// TestFallback_SuppressedByGuard は上限到達時に OpenAI を呼ばずローカルのエラーを返すことを
// 検証する（#1293 の Hard Limit）。
func TestFallback_SuppressedByGuard(t *testing.T) {
	localURL, fbURL, _, fbHits, _ := localAndFallbackServers(t, http.StatusInternalServerError)

	guard := &stubGuard{allow: false, reason: "日次コスト上限に到達"}
	tr := &fallbackTransport{
		primaryBaseURL: localURL, fallbackBaseURL: fbURL,
		fallbackKey: "sk-real", guard: guard, system: "text",
	}
	resp, err := roundTripLocal(t, tr, localURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500（ローカルのエラーがそのまま返るべき）", resp.StatusCode)
	}
	if n := atomic.LoadInt32(fbHits); n != 0 {
		t.Errorf("上限到達なのに OpenAI を呼んだ（%d 回）", n)
	}
	if guard.calls != 1 {
		t.Errorf("ガード参照回数=%d want 1", guard.calls)
	}
}

// TestFallback_DisabledByEnv は OPENAI_FALLBACK_ENABLED=false でトランスポート自体を
// 組まないことを検証する。
func TestFallback_DisabledByEnv(t *testing.T) {
	localURL, fbURL, _, fbHits, _ := localAndFallbackServers(t, http.StatusInternalServerError)

	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("OPENAI_FALLBACK_ENABLED", "false")
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", localURL)
	t.Setenv("AI_FALLBACK_BASE_URL", fbURL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cli.fallbacks) != 0 {
		t.Fatalf("fallback が組まれている: %v", cli.fallbacks)
	}
	if n := atomic.LoadInt32(fbHits); n != 0 {
		t.Errorf("無効化したのに OpenAI を呼んだ（%d 回）", n)
	}
}

// TestFallback_NotConfiguredWithoutKey は実キーが無ければフォールバックを組まないことを検証する。
// 逃げ先が無いので、組んでも 401 を1回増やすだけになる。
func TestFallback_NotConfiguredWithoutKey(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("AI_TEXT_PROVIDER", "local")
	t.Setenv("AI_TEXT_BASE_URL", "http://localhost:11434/v1")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cli.fallbacks) != 0 {
		t.Fatalf("キー無しで fallback が組まれている: %v", cli.fallbacks)
	}
}

// TestFallback_NotConfiguredForOpenAIPrimary は推論先が OpenAI 本家のときは
// フォールバックを組まないことを検証する（本家から本家へ逃げる意味がない）。
func TestFallback_NotConfiguredForOpenAIPrimary(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cli.fallbacks) != 0 {
		t.Fatalf("OpenAI 構成で fallback が組まれている: %v", cli.fallbacks)
	}
}

// TestFallback_ReplaysMultipartBody は音声アップロード（multipart）でも
// ボディを再送できることを検証する。
//
// ボディをバッファせずに再試行すると、2回目のリクエストが空になって
// フォールバックが常に失敗する。
func TestFallback_ReplaysMultipartBody(t *testing.T) {
	var fbBody int64
	var fbHits int32
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer local.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fbHits, 1)
		fbBody = r.ContentLength
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer fallback.Close()

	clearAIEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-real")
	t.Setenv("AI_AUDIO_PROVIDER", "local")
	t.Setenv("AI_AUDIO_BASE_URL", local.URL)
	t.Setenv("AI_FALLBACK_BASE_URL", fallback.URL)

	cli, err := NewFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	cli.SetFallbackGuard(&stubGuard{allow: true})

	audio := []byte(strings.Repeat("a", 2048))
	out, err := cli.Transcribe(context.Background(), audio, "a.webm")
	if err != nil {
		t.Fatalf("音声のフォールバックが失敗: %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %q, want ok", out)
	}
	if atomic.LoadInt32(&fbHits) != 1 {
		t.Fatalf("フォールバック先へのリクエスト=%d want 1", fbHits)
	}
	if fbBody <= int64(len(audio)) {
		t.Errorf("再送ボディが小さすぎる（音声が欠落している）: %d bytes", fbBody)
	}
}

// TestShouldFallback は再試行条件をテーブル駆動で固定する。
func TestShouldFallback(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
		want   bool
	}{
		{name: "接続エラーは再試行", err: errors.New("dial tcp: connection refused"), want: true},
		{name: "500は再試行", status: http.StatusInternalServerError, want: true},
		{name: "502は再試行", status: http.StatusBadGateway, want: true},
		{name: "503は再試行", status: http.StatusServiceUnavailable, want: true},
		{name: "429は再試行", status: http.StatusTooManyRequests, want: true},
		{name: "400は再試行しない", status: http.StatusBadRequest, want: false},
		{name: "401は再試行しない", status: http.StatusUnauthorized, want: false},
		{name: "404は再試行しない", status: http.StatusNotFound, want: false},
		{name: "200は再試行しない", status: http.StatusOK, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *http.Response
			if tt.status != 0 {
				resp = &http.Response{StatusCode: tt.status}
			}
			if got := shouldFallback(resp, tt.err); got != tt.want {
				t.Fatalf("shouldFallback() = %v, want %v", got, tt.want)
			}
		})
	}
}
