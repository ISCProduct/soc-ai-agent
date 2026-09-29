package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordedRequest は スタブが受け取ったリクエストのうち検証に使う部分。
type recordedRequest struct {
	TextFormat   string
	SystemPrompt string
}

// newResponsesStub は /responses のスタブを立てる。
// handler が非空の本文を返したらそれを 200 で返し、空文字列なら status + errBody を返す。
// 受け取ったリクエストは順に記録する。
func newResponsesStub(t *testing.T, handle func(req recordedRequest) (okBody string, status int, errBody string)) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text struct {
				Format struct {
					Type string `json:"type"`
				} `json:"format"`
			} `json:"text"`
			Input []struct {
				Role    string `json:"role"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		req := recordedRequest{TextFormat: body.Text.Format.Type}
		for _, in := range body.Input {
			if in.Role == "system" && len(in.Content) > 0 {
				req.SystemPrompt = in.Content[0].Text
			}
		}
		mu.Lock()
		got = append(got, req)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		okBody, status, errBody := handle(req)
		if okBody != "" {
			_, _ = w.Write([]byte(okBody))
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(errBody))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), got...)
	}
}

// formatsOf は記録したリクエストの text.format.type を並べる。
func formatsOf(reqs []recordedRequest) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.TextFormat
	}
	return out
}

func wantFormats(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("送った text.format.type = %v, want %v", got, want)
	}
}

// stubBackoff は responsesBackoff を待ち時間ゼロへ差し替え、待った attempt を記録する。
// 「最後の試行のあとは待たない」を実時間ぬきで確かめるための唯一の手段。
func stubBackoff(t *testing.T) func() []int {
	t.Helper()
	var mu sync.Mutex
	var slept []int
	orig := responsesBackoff
	responsesBackoff = func(attempt int) time.Duration {
		mu.Lock()
		slept = append(slept, attempt)
		mu.Unlock()
		return 0
	}
	t.Cleanup(func() { responsesBackoff = orig })
	return func() []int {
		mu.Lock()
		defer mu.Unlock()
		return append([]int(nil), slept...)
	}
}

// TestResponsesJSONWithMaxTokens_4xxならtext_formatを落として1度だけ試す は、
// JSON mode が 4xx で拒否されたら形式なしで取得できることを固定する（#1595）。
//
// 判定はエラー本文の字句ではなく「2回投げて差分が text.format だけ」という構造。
// 文面に依存しないので、422 を返す FastAPI 系や素文で 400 を返す互換サーバも通る。
func TestResponsesJSONWithMaxTokens_4xxならtext_formatを落として1度だけ試す(t *testing.T) {
	const okBody = `{"output_text":"{\"summary\":\"ok\"}"}`

	// 字句判定では拾えなかった／拾いすぎていた本文をそのまま並べる。
	tests := map[string]struct {
		status int
		body   string
	}{
		"OpenAIのパラメータ非対応":       {http.StatusBadRequest, `{"error":{"message":"Unsupported parameter: 'text.format' is not supported with this model.","type":"invalid_request_error","param":"text.format"}}`},
		"互換サーバのpydantic422":     {http.StatusUnprocessableEntity, `{"detail":[{"loc":["body","text","format"],"msg":"extra fields not permitted","type":"value_error.extra"}]}`},
		"互換サーバのnot_implemented": {http.StatusBadRequest, `{"error":{"message":"text.format is not implemented in this server"}}`},
		"互換サーバの素文":              {http.StatusBadRequest, `bad request`},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			srv, requests := newResponsesStub(t, func(req recordedRequest) (string, int, string) {
				if req.TextFormat != "" {
					return "", tt.status, tt.body
				}
				return okBody, 0, ""
			})
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			start := time.Now()
			out, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini")
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("形式を落として成功するべき: %v", err)
			}
			if out != `{"summary":"ok"}` {
				t.Errorf("本文 = %q", out)
			}
			// 1回目は JSON mode、2回目は text.format なし。この差分が判定そのもの。
			wantFormats(t, formatsOf(requests()), []string{textFormatJSON, ""})
			// バックオフ（最短2秒）へ入っていないこと。
			if elapsed > time.Second {
				t.Errorf("待ってから返している（経過 %v）", elapsed)
			}
		})
	}
}

// TestResponsesJSONWithMaxTokens_形式が原因でない4xxは元のエラーを返す は、
// 形式を落としても直らない 4xx で JSON mode が黙って無効化されないことを固定する（#1595）。
//
// 字句判定だと、リクエストをエコーする互換サーバ（pydantic/FastAPI の既定動作）では
// payload 内の json_object がゲートを通って過剰一致し、指示遵守率が 100%→0% に戻る。
// 構造判定なら2回目も同じく失敗するので、そもそも起こらない。
func TestResponsesJSONWithMaxTokens_形式が原因でない4xxは元のエラーを返す(t *testing.T) {
	// どちらも本文にリクエスト payload をエコーしている（字句判定が過剰一致した形）。
	tests := map[string]string{
		"コンテキスト長超過": `{"error":{"message":"This model's maximum context length is 8192 tokens. Request: {'text': {'format': {'type': 'json_object'}}} has an unsupported length"}}`,
		"必須パラメータ不足": `{"error":{"message":"Missing required parameter: 'model'. Received invalid value for {'text': {'format': {'type': 'json_object'}}}"}}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
				return "", http.StatusBadRequest, body
			})
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			start := time.Now()
			_, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini")
			elapsed := time.Since(start)
			if err == nil {
				t.Fatal("エラーになるべき")
			}
			// 返すのは元のエラー（形式なしの2回目は原因の切り分け用でしかない）。
			if !strings.Contains(err.Error(), body) {
				t.Errorf("元のエラー本文が返っていない: %v", err)
			}
			// 確かめるのは1度だけ。以降の試行は打ち切る。
			wantFormats(t, formatsOf(requests()), []string{textFormatJSON, ""})
			if elapsed > time.Second {
				t.Errorf("待ってから返している（経過 %v）", elapsed)
			}
		})
	}
}

// TestResponsesJSONWithMaxTokens_429では形式を落とさない は、
// 再試行すれば成功しうるステータスで JSON mode を捨てないことを固定する（#1595）。
func TestResponsesJSONWithMaxTokens_429では形式を落とさない(t *testing.T) {
	slept := stubBackoff(t)
	srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return "", http.StatusTooManyRequests, `{"error":{"message":"Rate limit reached"}}`
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	if _, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "systemはJSONを求める", "user", 0.2, 500, "gpt-4o-mini"); err == nil {
		t.Fatal("エラーになるべき")
	}
	for i, format := range formatsOf(requests()) {
		if format != textFormatJSON {
			t.Errorf("%d回目で JSON mode が外れた（text.format.type = %q）", i+1, format)
		}
	}
	if got := len(slept()); got == 0 {
		t.Error("429 は再試行するべき（待ちが一度も入っていない）")
	}
}

// TestResponsesWithMaxTokens_4xxは待たずに1回で返す は、
// 投げ直しても結果が変わらない 4xx を待たずに返すことを固定する（#1595）。
//
// 従来は最初の50msで確定した失敗を、5回とも踏んでから 62秒後に返していた。
// リクエスト回数と経過時間の2つが、この PR の「62秒 → 即返し」そのもの。
func TestResponsesWithMaxTokens_4xxは待たずに1回で返す(t *testing.T) {
	srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return "", http.StatusBadRequest, `{"error":{"message":"Missing required parameter: 'model'."}}`
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	start := time.Now()
	_, err := cli.ResponsesWithMaxTokens(context.Background(), "system", "user", 0.2, 500, "gpt-4o-mini")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("エラーになるべき")
	}
	if got := len(requests()); got != 1 {
		t.Errorf("リクエスト回数 = %d, want 1（4xx を投げ直している）", got)
	}
	if elapsed > time.Second {
		t.Errorf("待ってから返している（経過 %v）", elapsed)
	}
}

// TestResponsesWithMaxTokens_最後の試行のあとは待たない は、
// 5回目の失敗のあとに待ってから失敗を返していた無駄を固定する（#1595）。
//
// 実時間で見ると 30秒 vs 62秒 の差になって CI を殺すので、待った回数で見る。
func TestResponsesWithMaxTokens_最後の試行のあとは待たない(t *testing.T) {
	slept := stubBackoff(t)
	srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return "", http.StatusBadGateway, `{"error":{"message":"upstream error"}}`
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	if _, err := cli.ResponsesWithMaxTokens(context.Background(), "system", "user", 0.2, 500, "gpt-4o-mini"); err == nil {
		t.Fatal("エラーになるべき")
	}
	if got := len(requests()); got != 5 {
		t.Fatalf("リクエスト回数 = %d, want 5", got)
	}
	// 待つのは試行の「間」だけ。5回目のあとに待ったら 4 が 5 になる。
	if got := slept(); len(got) != 4 {
		t.Errorf("待った回数 = %d (%v), want 4（最後の試行のあとにも待っている）", len(got), got)
	}
}

// TestResponsesWithMaxTokens_待機中にコンテキストが切れたら即座に返す は、
// バックオフを time.Sleep で待っていたせいで打ち切れなかった空転を固定する（#1595）。
//
// 実時間で見ているのは、time.Sleep へ戻すと 62秒かかるのがここでしか現れないから。
// この経路が効くのは期限付きで呼ぶ interview_turn_question_plan.go:129 側で、
// context.Background() で呼ぶ resume_review.go 側には届かない（そちらは上の 4xx 即返しが担当）。
func TestResponsesWithMaxTokens_待機中にコンテキストが切れたら即座に返す(t *testing.T) {
	srv, _ := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return "", http.StatusBadGateway, `{"error":{"message":"upstream error"}}`
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := cli.ResponsesWithMaxTokens(ctx, "system", "user", 0.2, 500, "gpt-4o-mini")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("エラーになるべき")
	}
	if elapsed > time.Second {
		t.Errorf("バックオフを待ち切っている（経過 %v）", elapsed)
	}
}

// TestResponsesJSONWithMaxTokens_プロンプトにjsonの語が無ければ足す は、
// json_object が要求する "json" の語をクライアント側で担保することを固定する（#1595）。
//
// 語が無いだけなら JSON mode 自体は動くので、形式を落として指示遵守率を捨てるのではなく
// 足りない語を足す。これでプロンプト編集で語が消えても壊れない（往復は増えない）。
func TestResponsesJSONWithMaxTokens_プロンプトにjsonの語が無ければ足す(t *testing.T) {
	srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return `{"output_text":"{}"}`, 0, ""
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	if _, err := cli.ResponsesJSONWithMaxTokens(context.Background(), "あなたは添削者です", "経歴を見て", 0.2, 500, "gpt-4o-mini"); err != nil {
		t.Fatalf("成功するべき: %v", err)
	}
	got := requests()
	if len(got) != 1 {
		t.Fatalf("リクエスト回数 = %d, want 1（語を足せば往復は増えない）", len(got))
	}
	if !strings.Contains(strings.ToLower(got[0].SystemPrompt), "json") {
		t.Errorf("system プロンプトに json の語が無い: %q", got[0].SystemPrompt)
	}
	if got[0].TextFormat != textFormatJSON {
		t.Errorf("text.format.type = %q, want %q", got[0].TextFormat, textFormatJSON)
	}
}

// TestResponsesJSONWithMaxTokens_プロンプトにjsonの語があれば足さない は、
// 呼び出し側が語を担保しているときにプロンプトを書き換えないことを固定する（#1595）。
func TestResponsesJSONWithMaxTokens_プロンプトにjsonの語があれば足さない(t *testing.T) {
	const systemPrompt = "出力は次のJSONのみ"
	srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
		return `{"output_text":"{}"}`, 0, ""
	})
	cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

	if _, err := cli.ResponsesJSONWithMaxTokens(context.Background(), systemPrompt, "user", 0.2, 500, "gpt-4o-mini"); err != nil {
		t.Fatalf("成功するべき: %v", err)
	}
	if got := requests(); got[0].SystemPrompt != systemPrompt {
		t.Errorf("system プロンプト = %q, want %q", got[0].SystemPrompt, systemPrompt)
	}
}
