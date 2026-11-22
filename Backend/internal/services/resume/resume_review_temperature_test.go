package resume

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"Backend/internal/openai"
)

// 履歴書の採点は同一入力での点のゆらぎを下げるため、面接レポートと同じ低い温度で呼ぶ（SOCAIAGENT-699）。
// Responses API には seed が無いので、温度だけをそろえる。
func TestRequestReviewJSON_採点は低い温度で呼ぶ(t *testing.T) {
	var mu sync.Mutex
	var temps []float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Temperature *float64 `json:"temperature"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &req)
		mu.Lock()
		if req.Temperature != nil {
			temps = append(temps, *req.Temperature)
		} else {
			temps = append(temps, -1)
		}
		mu.Unlock()
		_, _ = w.Write([]byte(responsesBody(t, `{"scores":{}}`, "")))
	}))
	defer srv.Close()

	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	if _, err := svc.requestReviewJSON(context.Background(), ReviewSystemPrompt, "本文", ""); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(temps) != 1 {
		t.Fatalf("呼び出し回数 = %d, want 1", len(temps))
	}
	// 0 にすると省略されて API 既定(1.0)になる経路があるため、0 より大きく 0.05 以下を求める
	if temps[0] <= 0 || temps[0] > 0.05 {
		t.Errorf("temperature = %v, want 0 < t <= 0.05", temps[0])
	}
}
