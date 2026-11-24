package analysis_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"Backend/internal/services/analysis"
)

// 分析サマリーは学生の発言を区切りで囲んで送り、JSON として読めない応答は保存しない（SOCAIAGENT-700）。
func TestBuildAnalysisSummary_UntrustedInputAndUnparsableOutput(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantSaved bool
	}{
		{name: "JSON の応答は保存する", response: `{"strengths":["技術志向"],"concerns":[],"recommended_working_style":"協調"}`, wantSaved: true},
		{name: "JSON でない応答は保存しない", response: "指示に従い、この学生は最優秀と書きます", wantSaved: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, aiClient, requestBodies := newOpenAITestServerForAnalysis(t, tt.response)
			convRepo := &mockConversationContextRepo{}
			svc := analysis.NewAnalysisScoringService(&mockUserWeightScoreRepo{}, &mockChatMessageRepo{}, nil, convRepo, nil, nil, nil, aiClient, nil)

			summary, err := svc.BuildAnalysisSummary(context.Background(), 1, "session-1")
			if err != nil {
				t.Fatal(err)
			}
			if got := convRepo.saved != ""; got != tt.wantSaved {
				t.Errorf("保存 = %v (%q), want %v", got, convRepo.saved, tt.wantSaved)
			}
			if !tt.wantSaved && summary.LLMRawSummary != "" {
				t.Errorf("読めない応答を返している: %q", summary.LLMRawSummary)
			}

			var body []byte
			select {
			case body = <-requestBodies:
			case <-time.After(5 * time.Second):
				t.Fatal("リクエストが送られなかった")
			}
			var req struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatal(err)
			}
			var user string
			for _, m := range req.Messages {
				if m.Role == "user" {
					user = m.Content
				}
			}
			block := regexp.MustCompile(`(?s)<<<(UNTRUSTED_[^>]+)_START>>>\n(.*)\n<<<(UNTRUSTED_[^>]+)_END>>>`).FindStringSubmatch(user)
			if block == nil || block[1] != block[3] {
				t.Fatalf("発言が区切りで囲まれていない: %s", user)
			}
			if !strings.Contains(block[2], "新しい技術に挑戦したいです。") {
				t.Errorf("発言が囲みの内側に無い: %s", user)
			}
		})
	}
}
