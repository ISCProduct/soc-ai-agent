package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
		// ローカル側に実キーが届いていないことを全テストで検証する。
		// 「1回目はダミーキー」が最も重要な不変条件（#1293）。
		if got := r.Header.Get("Authorization"); strings.Contains(got, "sk-") {
			t.Errorf("ローカル推論先に実キーらしい値が送られた: %q", got)
		}
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

// TestFallback_NoGuardIsDenied はガード未注入なら再試行しないことを検証する（#1293 レビュー）。
//
// NewFromEnv は cmd/server 以外の main（crawl/cli/api）からも呼ばれ、
// そこでは SetFallbackGuard が呼ばれていない。許可側に倒すと、
// バッチ実行中にローカルが落ちた瞬間から上限なしで従量課金が進む。
func TestFallback_NoGuardIsDenied(t *testing.T) {
	localURL, fbURL, localHits, fbHits, _ := localAndFallbackServers(t, http.StatusInternalServerError)

	tr := &fallbackTransport{
		primaryBaseURL:  localURL,
		fallbackBaseURL: fbURL,
		fallbackKey:     "sk-real",
		system:          "text",
		// guard は意図的に nil
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, localURL+"/chat/completions", strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("ローカルのレスポンスがそのまま返るべき: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500（ローカルのエラーをそのまま返す）", resp.StatusCode)
	}
	if got := atomic.LoadInt32(localHits); got != 1 {
		t.Errorf("ローカルへの試行 = %d, want 1", got)
	}
	if got := atomic.LoadInt32(fbHits); got != 0 {
		t.Errorf("ガード未注入なのに OpenAI を呼んだ（%d 回）", got)
	}
}

// TestRewriteToFallback_URL は /v1 接頭辞の差し替えを固定する（#1293 レビュー）。
//
// e2e テストは httptest のパス無し URL を使うため、この分岐が全く固定されていなかった。
func TestRewriteToFallback_URL(t *testing.T) {
	tests := []struct {
		name     string
		primary  string
		reqPath  string
		rawQuery string
		want     string
	}{
		{name: "ローカルに/v1なし", primary: "http://localhost:11434", reqPath: "/chat/completions", want: "https://api.openai.com/v1/chat/completions"},
		{name: "ローカルに/v1あり", primary: "http://localhost:11434/v1", reqPath: "/v1/chat/completions", want: "https://api.openai.com/v1/chat/completions"},
		{name: "ローカルに/v1と末尾スラッシュ", primary: "http://localhost:11434/v1/", reqPath: "/v1/chat/completions", want: "https://api.openai.com/v1/chat/completions"},
		{name: "クエリを保持", primary: "http://localhost:11434/v1", reqPath: "/v1/audio/transcriptions", rawQuery: "x=1&y=2", want: "https://api.openai.com/v1/audio/transcriptions?x=1&y=2"},
		{name: "ネストした接頭辞", primary: "http://localhost:8000/openai/v1", reqPath: "/openai/v1/embeddings", want: "https://api.openai.com/v1/embeddings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &fallbackTransport{
				primaryBaseURL:  tt.primary,
				fallbackBaseURL: defaultOpenAIBaseURL,
				fallbackKey:     "sk-real",
			}
			req := httptest.NewRequest(http.MethodPost, "http://ignored"+tt.reqPath, nil)
			req.URL.RawQuery = tt.rawQuery

			out, err := tr.rewriteToFallback(req, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := out.URL.String(); got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
			if out.Host != "api.openai.com" {
				t.Errorf("Host = %q, want api.openai.com", out.Host)
			}
			if got := out.Header.Get("Authorization"); got != "Bearer sk-real" {
				t.Errorf("Authorization = %q", got)
			}
		})
	}
}

// TestRewriteToFallback_Model はローカルのモデル名を OpenAI のモデル名へ差し替えることを検証する。
//
// .env.example が推奨する gpt-oss-20b 等は api.openai.com に存在しないため、
// 差し替えないとフォールバックが model_not_found で必ず失敗する（#1293 レビュー）。
func TestRewriteToFallback_Model(t *testing.T) {
	tests := []struct {
		name          string
		fallbackModel string
		contentType   string
		body          string
		wantModel     string
		wantUnchanged bool
	}{
		{name: "JSONのmodelを差し替える", fallbackModel: "gpt-4o-mini", contentType: "application/json", body: `{"model":"gpt-oss-20b","messages":[]}`, wantModel: "gpt-4o-mini"},
		{name: "未設定なら触らない", fallbackModel: "", contentType: "application/json", body: `{"model":"gpt-oss-20b"}`, wantUnchanged: true},
		{name: "multipartは触らない", fallbackModel: "whisper-1", contentType: "multipart/form-data; boundary=x", body: "--x\r\n", wantUnchanged: true},
		{name: "modelキーが無ければ触らない", fallbackModel: "gpt-4o-mini", contentType: "application/json", body: `{"input":"x"}`, wantUnchanged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &fallbackTransport{
				primaryBaseURL:  "http://localhost:11434/v1",
				fallbackBaseURL: defaultOpenAIBaseURL,
				fallbackKey:     "sk-real",
				fallbackModel:   tt.fallbackModel,
			}
			req := httptest.NewRequest(http.MethodPost, "http://localhost:11434/v1/chat/completions", nil)
			req.Header.Set("Content-Type", tt.contentType)

			out, err := tr.rewriteToFallback(req, []byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			sent, err := io.ReadAll(out.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantUnchanged {
				if string(sent) != tt.body {
					t.Errorf("ボディが書き換えられた: %s", sent)
				}
				return
			}
			var payload map[string]any
			if err := json.Unmarshal(sent, &payload); err != nil {
				t.Fatalf("送信ボディがJSONでない: %s", sent)
			}
			if payload["model"] != tt.wantModel {
				t.Errorf("model = %v, want %q", payload["model"], tt.wantModel)
			}
			if out.ContentLength != int64(len(sent)) {
				t.Errorf("ContentLength = %d, want %d（書き換え後の長さ）", out.ContentLength, len(sent))
			}
		})
	}
}

// TestLocalAttemptContext は1回目に持ち時間を使い切らせないことを検証する（#1293 レビュー）。
//
// ローカル推論の実際の障害は「遅い・返らない」が主。1回目が期限を使い切ると
// フォールバックは0バイトも送れずに終わる（ログだけ出て実際は届かない）。
func TestLocalAttemptContext(t *testing.T) {
	tr := &fallbackTransport{}

	t.Run("呼び出し側に期限があれば一部だけ渡す", func(t *testing.T) {
		parent, cancelParent := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelParent()
		ctx, cancel := tr.localAttemptContext(parent)
		defer cancel()

		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("1回目に期限が設定されていない")
		}
		remaining := time.Until(deadline)
		if remaining >= 10*time.Second {
			t.Errorf("1回目が持ち時間を使い切っている: %v", remaining)
		}
		if remaining <= 0 {
			t.Errorf("1回目の持ち時間が無い: %v", remaining)
		}
	})

	t.Run("期限が無くenvも無ければ制限しない", func(t *testing.T) {
		ctx, cancel := tr.localAttemptContext(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Error("正常な長時間生成を打ち切らないため、期限を付けてはいけない")
		}
	})

	t.Run("期限が無くenvがあればそれを使う", func(t *testing.T) {
		limited := &fallbackTransport{localAttemptTimeout: 5 * time.Second}
		ctx, cancel := limited.localAttemptContext(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); !ok {
			t.Error("AI_LOCAL_ATTEMPT_TIMEOUT_SECONDS 相当の期限が効いていない")
		}
	})
}

// TestFallback_NotOnCallerCancel は呼び出し側のキャンセルで再試行しないことを検証する。
// 投げても無駄で、ガードの分間カウンタだけ消費する（#1293 レビュー）。
func TestFallback_NotOnCallerCancel(t *testing.T) {
	localURL, fbURL, _, fbHits, _ := localAndFallbackServers(t, http.StatusInternalServerError)
	guard := &stubGuard{allow: true}
	tr := &fallbackTransport{
		primaryBaseURL:  localURL,
		fallbackBaseURL: fbURL,
		fallbackKey:     "sk-real",
		guard:           guard,
		system:          "text",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 呼び出し側がすでに終了している
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, localURL+"/chat/completions", strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}

	_, _ = tr.RoundTrip(req)
	if got := atomic.LoadInt32(fbHits); got != 0 {
		t.Errorf("呼び出し側キャンセル後に OpenAI を呼んだ（%d 回）", got)
	}
}

// TestValidatedFallbackBaseURL は実キーを送る宛先の検証を固定する（#1293 レビュー）。
func TestValidatedFallbackBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "未設定はOpenAI本家", raw: "", want: defaultOpenAIBaseURL},
		{name: "httpsは許可", raw: "https://gw.example.com/v1", want: "https://gw.example.com/v1"},
		{name: "末尾スラッシュを剥がす", raw: "https://gw.example.com/v1/", want: "https://gw.example.com/v1"},
		{name: "平文httpの外部ホストは拒否", raw: "http://10.0.0.5/v1", wantErr: true},
		{name: "ループバックのhttpは許可", raw: "http://127.0.0.1:4000/v1", want: "http://127.0.0.1:4000/v1"},
		{name: "localhostのhttpは許可", raw: "http://localhost:4000/v1", want: "http://localhost:4000/v1"},
		{name: "scheme無しは拒否", raw: "gw.example.com:9999", wantErr: true},
		{name: "host空は拒否", raw: "https://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validatedFallbackBaseURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Errorf("エラーを期待したが %q が通った", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("= %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFallback_LocalResponseBodyIsFullyReadable は正常なローカル応答が
// 途中で切れないことを検証する（#1293 レビュー F-1）。
//
// 1回目の ctx は resp.Body の寿命を握っている。RoundTrip 直後にキャンセルすると
// バッファ済みの分（既定4KB）までしか読めず、正常な応答が切れる。
// 実アプリの応答は 18KB 程度、TTS の音声はさらに大きいので必ず踏む。
func TestFallback_LocalResponseBodyIsFullyReadable(t *testing.T) {
	const size = 512 * 1024
	payload := strings.Repeat("x", size)

	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(local.Close)

	tr := &fallbackTransport{
		primaryBaseURL:  local.URL,
		fallbackBaseURL: defaultOpenAIBaseURL,
		fallbackKey:     "sk-real",
		guard:           &stubGuard{allow: true},
		system:          "text",
	}

	// 呼び出し側に期限を持たせる（localAttemptContext が独自 ctx を張る条件）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, local.URL+"/chat/completions", strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	got, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("ボディを最後まで読めない（1回目のctxが早くキャンセルされている）: %v (読めたのは %d バイト)", readErr, len(got))
	}
	if len(got) != size {
		t.Errorf("読めたバイト数 = %d, want %d", len(got), size)
	}
}

// TestFallback_SuppressedResponseBodyIsReadable は上限到達で縮退したときに
// 返すローカルのエラーレスポンスも最後まで読めることを検証する（#1293 レビュー F-2）。
//
// ここが切れると、呼び出し元は「上限到達で縮退した」ではなく
// context canceled を見ることになり、運用の切り分けができない。
func TestFallback_SuppressedResponseBodyIsReadable(t *testing.T) {
	const size = 64 * 1024
	payload := strings.Repeat("e", size)

	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(local.Close)

	tr := &fallbackTransport{
		primaryBaseURL:  local.URL,
		fallbackBaseURL: defaultOpenAIBaseURL,
		fallbackKey:     "sk-real",
		guard:           &stubGuard{allow: false, reason: "日次コスト上限に到達"},
		system:          "text",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, local.URL+"/chat/completions", strings.NewReader(`{"model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	got, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("縮退時のローカルエラーを最後まで読めない: %v (読めたのは %d バイト)", readErr, len(got))
	}
	if len(got) != size {
		t.Errorf("読めたバイト数 = %d, want %d", len(got), size)
	}
}

// TestSetupFallbacks_DefaultFallbackModel は OpenAI 本家へ逃げるときに
// モデル名の既定値が入ることを検証する（#1293 レビュー F-6）。
//
// 既定が無いと .env.example の推奨構成（gpt-oss-20b 等）のままフォールバックしても
// model_not_found で必ず失敗し、機構はあるが既定で無効、という状態になる。
func TestSetupFallbacks_DefaultFallbackModel(t *testing.T) {
	tests := []struct {
		name           string
		fallbackURL    string
		envModel       string
		wantTextModel  string
		wantEmbedModel string
	}{
		{name: "本家宛は既定値が入る", wantTextModel: "gpt-4o-mini", wantEmbedModel: "text-embedding-3-small"},
		{name: "envが優先される", envModel: "gpt-4.1-mini", wantTextModel: "gpt-4.1-mini", wantEmbedModel: "text-embedding-3-small"},
		{
			// 互換ゲートウェイはモデル名を勝手に決められない
			name:        "ゲートウェイ宛は空のまま",
			fallbackURL: "https://gw.example.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAIEnv(t)
			t.Setenv("OPENAI_API_KEY", "sk-real")
			t.Setenv("AI_TEXT_PROVIDER", "local")
			t.Setenv("AI_TEXT_BASE_URL", "http://localhost:11434/v1")
			t.Setenv("AI_FALLBACK_BASE_URL", tt.fallbackURL)
			t.Setenv("AI_FALLBACK_MODEL", tt.envModel)

			cli, err := NewFromEnv("")
			if err != nil {
				t.Fatal(err)
			}
			if got := cli.fallbacks["text"]; got == nil {
				t.Fatal("text 系統にフォールバックが設定されていない")
			} else if got.fallbackModel != tt.wantTextModel {
				t.Errorf("text fallbackModel = %q, want %q", got.fallbackModel, tt.wantTextModel)
			}
			if got := cli.fallbacks["embedding"]; got != nil && got.fallbackModel != tt.wantEmbedModel {
				t.Errorf("embedding fallbackModel = %q, want %q", got.fallbackModel, tt.wantEmbedModel)
			}
		})
	}
}

// TestAllowsRealKey は実キーを送ってよい宛先の判定を固定する（#1293 レビュー F-5）。
// keyFor と validatedFallbackBaseURL がこの1つの判定を共有する。
func TestAllowsRealKey(t *testing.T) {
	tests := []struct {
		baseURL string
		want    bool
	}{
		{baseURL: "https://api.openai.com/v1", want: true},
		{baseURL: "https://gw.example.com/v1", want: true},
		{baseURL: "http://localhost:11434/v1", want: true},
		{baseURL: "http://127.0.0.1:4000/v1", want: true},
		{baseURL: "http://[::1]:4000/v1", want: true},
		{baseURL: "http://10.0.0.5/v1", want: false},
		{baseURL: "http://litellm:4000/v1", want: false},
		{baseURL: "ftp://example.com", want: false},
		{baseURL: "gw.example.com:9999", want: false},
		{baseURL: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			if got := allowsRealKey(tt.baseURL); got != tt.want {
				t.Errorf("allowsRealKey(%q) = %v, want %v", tt.baseURL, got, tt.want)
			}
		})
	}
}
