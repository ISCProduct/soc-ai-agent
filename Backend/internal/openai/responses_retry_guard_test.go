package openai

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Responses / ResponsesWithTemperature のリトライループの回帰テスト（#1605）。
//
// この2つは responsesWithMaxTokens（#1595 で修正済み）と同じ構造だが、
// 再試行可否の判定・最終試行後の待ち省略・ctx 連動のどれも無かった。
// スリープは 2+4+8+16+32 = 62秒で、400 を5回踏んでから失敗していた。
//
// 2つは同じループなので、呼び出し口だけ差し替えて同じ検証を回す。
// responsesWithMaxTokens 側は responses_json_fallback_test.go が既に固定している。
type retryTarget struct {
	name string
	call func(cli *Client, ctx context.Context) (string, error)
}

func retryTargets() []retryTarget {
	return []retryTarget{
		{
			name: "Responses",
			call: func(cli *Client, ctx context.Context) (string, error) {
				return cli.Responses(ctx, "input", "gpt-4o-mini")
			},
		},
		{
			name: "ResponsesWithTemperature",
			call: func(cli *Client, ctx context.Context) (string, error) {
				return cli.ResponsesWithTemperature(ctx, "system", "user", 0.2, "gpt-4o-mini")
			},
		},
	}
}

// 再試行しても結果が変わらない 4xx は、待たずに1回で返す。
//
// 429 以外の 4xx は投げ直しても同じ応答になる。待つのは利用者の時間を
// 捨てているだけで、学生が同期で待つ診断チャットがそのぶん固まる。
func TestResponses_再試行不可な4xxは待たずに1回で返す(t *testing.T) {
	statuses := map[string]int{
		"400 不正なリクエスト": http.StatusBadRequest,
		"401 認証エラー":    http.StatusUnauthorized,
		"404 モデルが無い":   http.StatusNotFound,
		"422 検証エラー":    http.StatusUnprocessableEntity,
	}
	for _, target := range retryTargets() {
		for name, status := range statuses {
			t.Run(target.name+"/"+name, func(t *testing.T) {
				srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
					return "", status, `{"error":{"message":"nope"}}`
				})
				slept := stubBackoff(t)
				cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

				if _, err := target.call(cli, context.Background()); err == nil {
					t.Fatal("エラーを返すべき")
				}
				if n := len(requests()); n != 1 {
					t.Errorf("リクエスト回数 = %d, want 1（投げ直しても同じ結果なので再試行しない）", n)
				}
				if s := slept(); len(s) != 0 {
					t.Errorf("バックオフへ入っている（attempt %v）。待ってから失敗を返すだけになる", s)
				}
			})
		}
	}
}

// 429 と 5xx は再試行する。上のテストが「何も再試行しない」実装でも通らないようにする。
func TestResponses_429と5xxは再試行する(t *testing.T) {
	statuses := map[string]int{
		"429 レート制限":  http.StatusTooManyRequests,
		"500 サーバエラー": http.StatusInternalServerError,
		"503 一時停止":   http.StatusServiceUnavailable,
	}
	for _, target := range retryTargets() {
		for name, status := range statuses {
			t.Run(target.name+"/"+name, func(t *testing.T) {
				srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
					return "", status, `{"error":{"message":"retry later"}}`
				})
				stubBackoff(t)
				cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

				if _, err := target.call(cli, context.Background()); err == nil {
					t.Fatal("エラーを返すべき")
				}
				if n := len(requests()); n != 5 {
					t.Errorf("リクエスト回数 = %d, want 5（再試行の上限まで試す）", n)
				}
			})
		}
	}
}

// 最後の試行のあとに待つ意味は無い。待ってから失敗を返すだけで、
// 62秒のうち最後の32秒は完全な無駄だった。
func TestResponses_最後の試行のあとは待たない(t *testing.T) {
	for _, target := range retryTargets() {
		t.Run(target.name, func(t *testing.T) {
			srv, _ := newResponsesStub(t, func(recordedRequest) (string, int, string) {
				return "", http.StatusInternalServerError, `{"error":{"message":"boom"}}`
			})
			slept := stubBackoff(t)
			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")

			if _, err := target.call(cli, context.Background()); err == nil {
				t.Fatal("エラーを返すべき")
			}
			// 5回試すなら待つのは1〜4回目のあとだけ。5回目のあとは待たない。
			got := slept()
			if len(got) != 4 {
				t.Fatalf("待った回数 = %d (attempt %v), want 4", len(got), got)
			}
			for i, attempt := range got {
				if attempt != i+1 {
					t.Errorf("待った attempt = %v, want [1 2 3 4]", got)
					break
				}
			}
		})
	}
}

// 待っている間に呼び出し側が諦めたら即座に返す。
// time.Sleep は ctx を見ないので、学生が画面を閉じてもサーバは最大62秒寝ていた。
func TestResponses_待機中にctxがキャンセルされたら即返す(t *testing.T) {
	for _, target := range retryTargets() {
		t.Run(target.name, func(t *testing.T) {
			srv, requests := newResponsesStub(t, func(recordedRequest) (string, int, string) {
				return "", http.StatusInternalServerError, `{"error":{"message":"boom"}}`
			})
			// 待ち時間を実際に発生させる。ここを0にすると ctx を見ていない実装でも通る。
			orig := responsesBackoff
			responsesBackoff = func(int) time.Duration { return 10 * time.Second }
			t.Cleanup(func() { responsesBackoff = orig })

			cli := NewWithBaseURL(srv.URL, "gpt-4o-mini")
			ctx, cancel := context.WithCancel(context.Background())
			// 1回目の応答が返ってバックオフへ入ったころにキャンセルする。
			go func() {
				time.Sleep(200 * time.Millisecond)
				cancel()
			}()

			start := time.Now()
			if _, err := target.call(cli, ctx); err == nil {
				t.Fatal("エラーを返すべき")
			}
			if elapsed := time.Since(start); elapsed > 3*time.Second {
				t.Errorf("キャンセル後も待っている（経過 %v）", elapsed)
			}
			if n := len(requests()); n != 1 {
				t.Errorf("リクエスト回数 = %d, want 1（キャンセル後は投げない）", n)
			}
		})
	}
}
