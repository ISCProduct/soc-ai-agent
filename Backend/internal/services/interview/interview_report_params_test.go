package interview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// 採点の呼び出しが exported 定数のパラメータでワイヤまで届くことを固定する（SOCAIAGENT-513）。
func TestGenerateReport_SendsDeterminismParams(t *testing.T) {
	var got struct {
		Temperature float64 `json:"temperature"`
		Seed        *int    `json:"seed"`
		MaxTokens   int     `json:"max_completion_tokens"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "{}"}}}})
	}))
	defer srv.Close()

	svc := NewInterviewService(newSessionRepoStub(&models.InterviewSession{ID: 7, UserID: 3, Status: "finished", Language: "ja"}),
		&utterRepoStub{utterances: evidenceTestUtterances()}, &reportRepoStub{}, nil, nil, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"), nil)
	// 中身が不正でも1回目のリクエストは送られる。結果はここでは見ない。
	_ = svc.generateReport(context.Background(), 7)

	tests := []struct {
		name string
		ok   bool
	}{
		{"temperature", float32(got.Temperature) == ReportTemperature && got.Temperature > 0},
		{"seed", got.Seed != nil && *got.Seed == ReportSeed},
		{"max_tokens", got.MaxTokens == ReportMaxTokens},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.ok {
				t.Errorf("リクエストに定数が反映されていない: %+v", got)
			}
		})
	}
}
