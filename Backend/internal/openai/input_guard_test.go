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

// 面接官の応答の呼び出しは、発話の履歴（学生の発話と面接官の応答）に伏字をかける。
// 面接官のシステムプロンプトは固定文（学生の名前を呼びかけに使う機能を含む）なので、変えない。
func TestInputGuard_ChatInterviewMasksHistoryButNotSystemPrompt(t *testing.T) {
	const secret = "山田太郎"
	guard := InputGuard(func(text string) (string, error) { return strings.ReplaceAll(text, secret, "[NAME]"), nil })
	srv, bodies := newBodyRecordingServer(t)
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	history := []map[string]string{
		{"role": "assistant", "content": "自己紹介をお願いします。"},
		{"role": "user", "content": "山田太郎です。チーム開発をしました。"},
	}
	if _, err := cli.ChatInterview(WithInputGuard(context.Background(), guard), "面接官です。応募者は"+secret+"さんです。", history); err != nil {
		t.Fatalf("呼び出しに失敗: %v", err)
	}
	got := bodies()
	if len(got) != 1 {
		t.Fatalf("リクエスト数 = %d, want 1", len(got))
	}
	if !strings.Contains(got[0], "チーム開発をしました") {
		t.Errorf("発話が消えた: %s", got[0])
	}
	if !strings.Contains(got[0], "[NAME]です") {
		t.Errorf("履歴の氏名が伏字になっていない: %s", got[0])
	}
	if !strings.Contains(got[0], "応募者は"+secret+"さんです") {
		t.Errorf("システムプロンプトが変わった: %s", got[0])
	}

	// ガードが失敗したら送らない
	srv2, bodies2 := newBodyRecordingServer(t)
	cli2 := NewWithBaseURL(srv2.URL, "gpt-4o-mini")
	bad := WithInputGuard(context.Background(), func(string) (string, error) { return "", errors.New("x") })
	if _, err := cli2.ChatInterview(bad, "sys", history); !errors.Is(err, ErrInputGuardFailed) {
		t.Errorf("err = %v, want ErrInputGuardFailed", err)
	}
	if n := len(bodies2()); n != 0 {
		t.Errorf("失敗なのに %d 件送られた", n)
	}
}
