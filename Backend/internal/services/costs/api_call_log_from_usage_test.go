package costs

import (
	"testing"
	"time"

	openaiPkg "Backend/internal/openai"
)

// 工程と版が api_call_logs へ入ることを検証する（#1778）。
// 工程が空の呼び出しは unknown として残し、未計測の経路を数えられるようにする。
func TestAPICallLogFromUsage_PromptVersion(t *testing.T) {
	tests := []struct {
		name        string
		stage       string
		version     string
		wantStage   string
		wantVersion string
	}{
		{name: "工程と版をそのまま記録する", stage: "score_evaluation", version: "score-t1", wantStage: "score_evaluation", wantVersion: "score-t1"},
		{name: "工程が空ならunknown", wantStage: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiCallLogFromUsage(openaiPkg.Usage{PromptStage: tt.stage, PromptVersion: tt.version}, time.Now())

			if got.PromptStage != tt.wantStage || got.PromptVersion != tt.wantVersion {
				t.Fatalf("got=(%q,%q), want (%q,%q)", got.PromptStage, got.PromptVersion, tt.wantStage, tt.wantVersion)
			}
		})
	}
}
