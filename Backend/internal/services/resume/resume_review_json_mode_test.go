package resume

import (
	"encoding/json"
	"strings"
	"testing"
)

// reviewJSONModeType はレビュー生成が要求する text.format.type（#1583）。
// 指定が外れるとモデルはコードフェンスや前置きを付けて返し、decodeJSON の
// 復旧処理に依存した状態（指示遵守率0%）へ戻る。
const reviewJSONModeType = "json_object"

// TestRequestReviewJSON_JSONmodeで呼ぶ は、レビュー生成のすべての呼び出しが
// JSON mode であることを検証する。初回とやり直しは別プロンプトなので両方見る。
func TestRequestReviewJSON_JSONmodeで呼ぶ(t *testing.T) {
	tests := []struct {
		name      string
		bodies    []string
		wantCalls int
	}{
		{
			name:      "初回で成立する",
			bodies:    []string{reviewJSON(t, rubricScores(5, 4, 4, 5, 4))},
			wantCalls: 1,
		},
		{
			// 指摘が3件未満だと別プロンプトでやり直す。そちらも JSON mode でなければ
			// 「やり直しだけ素の JSON でない」という分かりにくい形で再発する。
			name: "指摘不足でやり直す",
			bodies: []string{
				reviewJSONItems(t, rubricScores(5, 4, 4, 5, 4), "指摘", blockTexts[:1]),
				reviewJSONItems(t, rubricScores(5, 4, 4, 5, 4), "指摘", blockTexts[:3]),
			},
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodies := make([]string, 0, len(tt.bodies))
			for _, b := range tt.bodies {
				bodies = append(bodies, responsesBody(t, b, ""))
			}
			svc, stub := newReviewService(t, bodies...)

			if _, _, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, ""); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			formats := stub.requestedTextFormats()
			if len(formats) != tt.wantCalls {
				t.Fatalf("AI 呼び出し回数 = %d, want %d", len(formats), tt.wantCalls)
			}
			for i, got := range formats {
				if got != reviewJSONModeType {
					t.Errorf("%d回目の text.format.type = %q, want %q", i+1, got, reviewJSONModeType)
				}
			}
		})
	}
}

// TestRequestReviewJSON_プロンプトにJSONの語がある は JSON mode の前提を固定する。
// json_object を指定したプロンプトに "JSON" の語が無いと API がエラーを返すため、
// system プロンプトから語を消すとレビューが丸ごと失敗する。
func TestRequestReviewJSON_プロンプトにJSONの語がある(t *testing.T) {
	if !strings.Contains(ReviewSystemPrompt, "JSON") {
		t.Errorf("ReviewSystemPrompt に \"JSON\" が無い: %q", ReviewSystemPrompt)
	}
	if !strings.Contains(reviewRetrySystemPrompt, "JSON") {
		t.Errorf("reviewRetrySystemPrompt に \"JSON\" が無い: %q", reviewRetrySystemPrompt)
	}
}

// TestDecodeJSON_復旧経路 は decodeJSON の3つの入力を固定する。
//
// JSON mode を入れた後にモデルが返すのは1つ目（素の JSON）だけで、
// 2つ目・3つ目は JSON mode を外したときに戻ってくる入力である。
// 3つ目は「前置きに '{' が含まれると誤った範囲を切り出す」という
// 復旧処理の限界そのもので、保険に依存できない理由になっている。
func TestDecodeJSON_復旧経路(t *testing.T) {
	const bare = `{"summary":"確認しました"}`

	tests := []struct {
		name string
		raw  string
		// wantRecovery は '{' 〜 '}' の切り出しを通るか（素の JSON なら false）。
		wantRecovery bool
		wantErr      bool
		wantSummary  string
	}{
		{
			name:         "JSON mode の出力（素のJSON）は復旧処理を通らない",
			raw:          bare,
			wantRecovery: false,
			wantSummary:  "確認しました",
		},
		{
			name:         "コードフェンス付きは復旧処理で読める",
			raw:          "```json\n" + bare + "\n```",
			wantRecovery: true,
			wantSummary:  "確認しました",
		},
		{
			name: "前置きに { があると誤った範囲を切り出して失敗する",
			raw:  "以下のとおりです{参考}\n" + bare,
			// 最初の '{' が前置き側なので、切り出し結果が JSON にならない
			wantRecovery: true,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 「復旧処理を通ったか」は、最初の Unmarshal が成功するかで決まる。
			var direct map[string]any
			gotRecovery := json.Unmarshal([]byte(strings.TrimSpace(tt.raw)), &direct) != nil
			if gotRecovery != tt.wantRecovery {
				t.Errorf("復旧処理を通ったか = %v, want %v", gotRecovery, tt.wantRecovery)
			}

			var out aiReviewResponse
			err := decodeJSON(tt.raw, &out)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("エラーになるべき（summary=%q）", out.Summary)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", out.Summary, tt.wantSummary)
			}
		})
	}
}
