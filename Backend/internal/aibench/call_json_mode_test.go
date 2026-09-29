package aibench

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ハーネスは本番と同じ条件で測らないと数字の意味が無い。履歴書レビューは
// JSON mode（text.format.type=json_object / #1583）で呼ぶので、ここが外れると
// 指示遵守率だけが本番と違う値になる。
const wantResponsesJSONFormat = "json_object"

// captureResponsesPayload は /responses へ届いたペイロードを1件返すスタブ。
func captureResponsesPayload(t *testing.T, got *map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, got); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"{}"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "テスト用")
}

// TestCallResponses_JSONmodeの指定 は jsonMode の on/off でペイロードが変わることを固定する。
func TestCallResponses_JSONmodeの指定(t *testing.T) {
	tests := []struct {
		name       string
		jsonMode   bool
		wantFormat string // "" は text を送らない
	}{
		{name: "jsonMode=false は text を送らない", jsonMode: false, wantFormat: ""},
		{name: "jsonMode=true は json_object を送る", jsonMode: true, wantFormat: wantResponsesJSONFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload map[string]any
			captureResponsesPayload(t, &payload)

			res := CallResponses(context.Background(), "gpt-4o-mini", "system", "user", 0.2, 500, tt.jsonMode)
			if res.Err != nil {
				t.Fatalf("呼び出しに失敗: %v", res.Err)
			}
			if got := responsesTextFormat(payload); got != tt.wantFormat {
				t.Errorf("text.format.type = %q, want %q", got, tt.wantFormat)
			}
		})
	}
}

// TestResumeTarget_JSONmodeで呼ぶ は履歴書レビューの評価が JSON mode を使うことを固定する。
// CallResponses の引数を false に戻すと落ちる。
func TestResumeTarget_JSONmodeで呼ぶ(t *testing.T) {
	var payload map[string]any
	captureResponsesPayload(t, &payload)

	target := newResumeTarget("gpt-4o-mini")
	target.Run(context.Background(), Case{
		ID:    "t1",
		Label: LabelMid,
		Input: Input{ResumeText: "売上を前年比120%に伸ばしました", JobTitle: "エンジニア", CandidateType: "new_grad"},
	})

	if got := responsesTextFormat(payload); got != wantResponsesJSONFormat {
		t.Errorf("text.format.type = %q, want %q", got, wantResponsesJSONFormat)
	}
}

// responsesTextFormat はペイロードの text.format.type を取り出す（無ければ空文字列）。
func responsesTextFormat(payload map[string]any) string {
	text, ok := payload["text"].(map[string]any)
	if !ok {
		return ""
	}
	format, ok := text["format"].(map[string]any)
	if !ok {
		return ""
	}
	t, _ := format["type"].(string)
	return t
}
