package aibench

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"Backend/internal/services/prompts"
)

// 評価ロジックは LLM を呼ばずに固定する。

func TestEvaluateChatAnswerResponse(t *testing.T) {
	tests := []struct {
		name           string
		res            CallResult
		wantBroken     bool
		wantReason     string
		wantScore      float64
		wantRaw        float64
		wantViolations []string
	}{
		{name: "正常", res: CallResult{Text: `{"score":72,"confidence":"high","explanation":"理由"}`}, wantScore: 0.72, wantRaw: 72},
		{name: "0点は欠落ではなく有効な点", res: CallResult{Text: `{"score":0,"confidence":"low"}`}, wantScore: 0, wantRaw: 0},
		{name: "出力上限到達", res: CallResult{Text: `{"score":7`, Truncated: true}, wantBroken: true, wantReason: BrokenTruncated},
		{name: "API失敗", res: CallResult{Err: errors.New("HTTP 500")}, wantBroken: true, wantReason: BrokenCallFailed},
		{name: "通信エラーは計測できず", res: CallResult{Err: errors.New("timeout"), Unmeasured: true}, wantBroken: true, wantReason: BrokenNetwork},
		{name: "JSONとして読めない", res: CallResult{Text: "評価できません"}, wantBroken: true, wantReason: BrokenJSONInvalid},
		{
			// 本番は Score int のまま Unmarshal するので、欠落は 0 点として採用される。
			name: "scoreが無いときは本番と同じく0点として数える", res: CallResult{Text: `{"confidence":"high"}`},
			wantScore: 0, wantRaw: 0, wantViolations: []string{"score_missing"},
		},
		{name: "scoreが小数なら読めない(本番のintでも読めない)", res: CallResult{Text: `{"score":72.5,"confidence":"high"}`}, wantBroken: true, wantReason: BrokenJSONInvalid},
		{
			// 本番は丸めて通す。ハーネスも同じ値を測り、起きたことは指示違反として残す。
			name: "100超は100へ丸める", res: CallResult{Text: `{"score":150,"confidence":"high"}`},
			wantScore: 1, wantRaw: 100, wantViolations: []string{"score_out_of_range"},
		},
		{
			name: "負の点は0へ丸める", res: CallResult{Text: `{"score":-5,"confidence":"low"}`},
			wantScore: 0, wantRaw: 0, wantViolations: []string{"score_out_of_range"},
		},
		{
			name: "confidenceが規定外", res: CallResult{Text: `{"score":50,"confidence":"very high"}`},
			wantScore: 0.5, wantRaw: 50, wantViolations: []string{"confidence_invalid"},
		},
		{
			// 他の target と違い、本番はコードフェンスや散文を剥がさずに Unmarshal する。
			// 読めない出力は本番でも捨てられるので、ここでは破損に数える。
			name: "散文で包まれたJSONは本番でも読めないので破損", res: CallResult{Text: "結果です\n```json\n" + `{"score":60,"confidence":"medium"}` + "\n```"},
			wantBroken: true, wantReason: BrokenJSONInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Case{ID: "ca-x", Label: LabelGood, Target: TargetChatAnswer, Input: Input{Question: "質問", Answer: "回答"}}
			o := evaluateChatAnswerResponse(newObservation(c), tt.res)
			if o.Broken != tt.wantBroken {
				t.Fatalf("Broken = %v, want %v (%s)", o.Broken, tt.wantBroken, o.Error)
			}
			if tt.wantBroken {
				if o.BrokenReason != tt.wantReason {
					t.Errorf("BrokenReason = %q, want %q", o.BrokenReason, tt.wantReason)
				}
				return
			}
			if o.Score != tt.wantScore || o.RawScore != tt.wantRaw {
				t.Errorf("Score = %v / Raw = %v, want %v / %v", o.Score, o.RawScore, tt.wantScore, tt.wantRaw)
			}
			if !slices.Equal(o.Violations, tt.wantViolations) {
				t.Errorf("Violations = %v, want %v", o.Violations, tt.wantViolations)
			}
		})
	}
}

// 定型文の概算トークン数が実測から離れると、実行前のコスト確認が実費と合わなくなる。
func TestChatAnswerPromptOverheadTokens(t *testing.T) {
	const q, a = "問", "答"
	got := ApproxTokensJA(prompts.AnswerQualitySystemPrompt+prompts.BuildAnswerQualityUserPrompt(q, a)) - ApproxTokensJA(q) - ApproxTokensJA(a)
	const tolerance = 40
	if diff := got - chatAnswerPromptOverheadTokens; diff > tolerance || diff < -tolerance {
		t.Fatalf("定型文の実測 %d トークンに対し chatAnswerPromptOverheadTokens=%d。測り直して更新すること", got, chatAnswerPromptOverheadTokens)
	}
}

func TestNewTargetChatAnswer(t *testing.T) {
	t.Setenv("AI_TEXT_MODEL", "")
	t.Setenv("OPENAI_MODEL", "")
	tgt, err := NewTarget(TargetChatAnswer, "")
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Name() != TargetChatAnswer || tgt.Model() != "gpt-4o-mini" {
		t.Errorf("Name=%q Model=%q", tgt.Name(), tgt.Model())
	}
	t.Setenv("AI_TEXT_MODEL", "model-a")
	if tgt, _ = NewTarget(TargetChatAnswer, ""); tgt.Model() != "model-a" {
		t.Errorf("AI_TEXT_MODEL を使うべき: %q", tgt.Model())
	}
	if tgt, _ = NewTarget(TargetChatAnswer, "override"); tgt.Model() != "override" {
		t.Errorf("引数が最優先: %q", tgt.Model())
	}
}

func TestEvaluateChatAnswerResponse_他フィールドの型エラーは本番と同じく破損(t *testing.T) {
	// 本番は specificity を int で受けるので、小数が来ると評価ごと捨てられる。
	o := evaluateChatAnswerResponse(newObservation(Case{ID: "x"}), CallResult{Text: `{"score":50,"confidence":"medium","specificity":2.5}`})
	if !o.Broken || o.BrokenReason != BrokenJSONInvalid {
		t.Errorf("Broken=%v reason=%q, want JSON不正", o.Broken, o.BrokenReason)
	}
}

// 本番と違う条件で呼ぶと、破損率も指示遵守率も別物を測ることになる。
// 本番側は answer_evaluator.go にリテラル(0.2 / 256)で埋まっていて同期が取れないため、
// ここが唯一の防波堤になる。
func TestChatAnswerTarget_Run呼び出し内容(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"score\":50,\"confidence\":\"medium\"}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "テスト用")

	tgt := newChatAnswerTarget("model-x")
	c := Case{ID: "ca-x", Label: LabelGood, Target: TargetChatAnswer, Input: Input{Question: "質問です", Answer: "回答です"}}
	o := tgt.Run(context.Background(), c)
	if o.Broken {
		t.Fatalf("破損: %s", o.Error)
	}
	if got["model"] != "model-x" || got["temperature"] != 0.2 || got["max_completion_tokens"] != float64(256) {
		t.Errorf("model=%v temperature=%v max_completion_tokens=%v", got["model"], got["temperature"], got["max_completion_tokens"])
	}
	if rf, _ := got["response_format"].(map[string]any); rf["type"] != "json_object" {
		t.Errorf("response_format = %v, want json_object", got["response_format"])
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d件, want 2", len(msgs))
	}
	sys, _ := msgs[0].(map[string]any)
	usr, _ := msgs[1].(map[string]any)
	// ユーザープロンプトは回答をノンス付きの区切りで囲む（#1636）ので毎回文字列が変わる。
	// 完全一致ではなく、本番の関数が作る構造（質問が先、回答を含む）で確かめる。
	usrText, _ := usr["content"].(string)
	if sys["content"] != prompts.AnswerQualitySystemPrompt {
		t.Error("システムプロンプトが prompts.AnswerQualitySystemPrompt と一致しない")
	}
	if !strings.HasPrefix(usrText, "【質問】\n質問です\n\n【回答】\n") || !strings.Contains(usrText, "回答です") {
		t.Errorf("ユーザープロンプトが BuildAnswerQualityUserPrompt の構造と違う: %q", usrText)
	}
}
