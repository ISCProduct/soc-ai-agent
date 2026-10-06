package resume

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"Backend/internal/openai"
)

// textFormatRejected400 は text.format を受け付けない互換サーバが返す 400。
// 文面はクライアント側の判定に使われない（判定は「再試行不可の 4xx かつ text.format を
// 送っていた」という構造だけ）ので、素文でも pydantic の 422 でも同じ経路を通る（#1595）。
const textFormatRejected400 = `{"error":{"message":"text.format is not supported by this server"}}`

// TestBuildReviewScoreItems_JSONmode非対応でもレビューが成立する は、
// JSON mode を拒否する API（ローカルLLM / OpenAI 互換実装）を相手にしても
// レビューが完成することを検証する（#1595）。
//
// 形式を落としたあとの本文は JSON mode 以前と同じくコードフェンス付きになるため、
// `decodeJSON` の '{' 〜 '}' 切り出し（#1583 の後も残している復旧経路）を通る。
// 退避だけ入れても復旧経路が死んでいたら意味が無いので、ここで通しで見る。
func TestBuildReviewScoreItems_JSONmode非対応でもレビューが成立する(t *testing.T) {
	var mu sync.Mutex
	var formats []string
	// 形式を落としたあとにモデルが返す形（JSON mode が無いと前置きやコードフェンスが付く）
	fenced := "```json\n" + reviewJSON(t, rubricScores(5, 4, 4, 5, 4)) + "\n```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text struct {
				Format struct {
					Type string `json:"type"`
				} `json:"format"`
			} `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("リクエストの解析に失敗: %v", err)
		}
		mu.Lock()
		formats = append(formats, req.Text.Format.Type)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if req.Text.Format.Type == reviewJSONModeType {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(textFormatRejected400))
			return
		}
		_, _ = w.Write([]byte(responsesBody(t, fenced, "")))
	}))
	t.Cleanup(srv.Close)

	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("形式を落としてレビューが成立するべき: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("指摘が1件も紐づいていない（復旧経路で読めていない）")
	}
	if review.Score == nil || *review.Score != 90 {
		t.Errorf("Score = %v, want 90（コードフェンス付きの本文が読めていない）", review.Score)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{reviewJSONModeType, ""}
	if len(formats) != len(want) {
		t.Fatalf("送った text.format.type = %v, want %v", formats, want)
	}
	for i := range want {
		if formats[i] != want[i] {
			t.Errorf("%d回目の text.format.type = %q, want %q", i+1, formats[i], want[i])
		}
	}
}
