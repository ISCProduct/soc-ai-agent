package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// 受け取ったリクエストの本文を記録する疑似サーバー。経路ごとに最小の正常応答を返す。
func newBodyRecordingServer(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/embeddings"):
			_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2],"index":0}]}`))
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{}"}}]}`))
		default:
			_, _ = w.Write([]byte(`{"output_text":"ok"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

// 学生の入力（ユーザープロンプト・埋め込み入力）には、ガードをかけてから送る。固定のシステムプロンプトは変えない。
func TestInputGuard_AppliedToEveryTextEntryPoint(t *testing.T) {
	const secret = "山田太郎"
	guard := InputGuard(func(text string) (string, error) { return strings.ReplaceAll(text, secret, "[NAME]"), nil })

	calls := map[string]func(cli *Client, ctx context.Context) error{
		"Responses": func(cli *Client, ctx context.Context) error {
			_, err := cli.Responses(ctx, "SYS 回答: "+secret)
			return err
		},
		"ResponsesWithTemperature": func(cli *Client, ctx context.Context) error {
			_, err := cli.ResponsesWithTemperature(ctx, "SYS", "回答: "+secret, 0.2)
			return err
		},
		"ResponsesWithMaxTokens": func(cli *Client, ctx context.Context) error {
			_, err := cli.ResponsesWithMaxTokens(ctx, "SYS", "回答: "+secret, 0.2, 100)
			return err
		},
		"ResponsesJSONWithMaxTokens": func(cli *Client, ctx context.Context) error {
			_, err := cli.ResponsesJSONWithMaxTokens(ctx, "SYS json", "回答: "+secret, 0.2, 100)
			return err
		},
		"ChatCompletionJSON": func(cli *Client, ctx context.Context) error {
			_, err := cli.ChatCompletionJSON(ctx, "SYS json", "回答: "+secret, 0.2, 100)
			return err
		},
		"Embedding": func(cli *Client, ctx context.Context) error {
			_, err := cli.Embedding(ctx, "回答: "+secret)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			srv, bodies := newBodyRecordingServer(t)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
			if err := call(cli, WithInputGuard(context.Background(), guard)); err != nil {
				t.Fatalf("呼び出しに失敗: %v", err)
			}
			got := bodies()
			if len(got) == 0 {
				t.Fatal("リクエストが送られていない")
			}
			for _, b := range got {
				if strings.Contains(b, secret) {
					t.Errorf("伏字にされないまま送られた: %s", b)
				}
				if !strings.Contains(b, "[NAME]") {
					t.Errorf("ガード後の文が送られていない: %s", b)
				}
			}
		})
	}
}

// ガードを作れない・かけられないときは、何も送らない。
func TestInputGuard_FailsClosed(t *testing.T) {
	guard := InputGuard(func(string) (string, error) { return "", errors.New("辞書を作れない") })
	srv, bodies := newBodyRecordingServer(t)
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	ctx := WithInputGuard(context.Background(), guard)

	_, errs := cli.Responses(ctx, "回答: 山田太郎")
	_, errc := cli.ChatCompletionJSON(ctx, "SYS json", "回答: 山田太郎", 0.2, 100)
	_, erre := cli.Embedding(ctx, "回答: 山田太郎")
	for name, err := range map[string]error{"Responses": errs, "ChatCompletionJSON": errc, "Embedding": erre} {
		if !errors.Is(err, ErrInputGuardFailed) {
			t.Errorf("%s: err = %v, want ErrInputGuardFailed", name, err)
		}
	}
	if n := len(bodies()); n != 0 {
		t.Errorf("ガード失敗なのに %d 件送られた", n)
	}
}

// ガードが無いコンテキスト（企業調査など、学生の文章を含まない呼び出し）は、何も変わらない。
func TestInputGuard_NoGuardLeavesInputUntouched(t *testing.T) {
	srv, bodies := newBodyRecordingServer(t)
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
	if _, err := cli.Responses(context.Background(), "https://example.com の企業情報"); err != nil {
		t.Fatal(err)
	}
	got := bodies()
	if len(got) != 1 || !strings.Contains(got[0], "https://example.com") {
		t.Errorf("入力が変わった: %v", got)
	}
}
