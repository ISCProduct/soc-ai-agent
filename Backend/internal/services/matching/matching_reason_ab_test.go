package matching

import (
	"Backend/internal/models"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/openai"
	"Backend/internal/services/prompts"
)

// #1976: マッチング理由は対象の利用者で A/B を割り当てる。チャット後・面接後のワーカーや
// バッチは context の実行主体なしで走るので、実行主体に頼ると経路で割当が変わる。
func TestGenerateMatchReason_ABは対象の利用者で割り当てる(t *testing.T) {
	t.Cleanup(prompts.RegisterCandidateBodies(prompts.StageMatchingReason, "v3", map[string]string{
		prompts.BodyMatchingReasonSystem: "候補のマッチング理由プロンプト",
	}))
	t.Setenv("PROMPT_AB_MATCHING_REASON", "v2:50,v3:50")
	t.Setenv("MATCHING_REASON_USE_AI", "true")

	users := map[string]uint{}
	for id := uint(1); len(users) < 2 && id < 1000; id++ {
		version, _ := prompts.AssignPromptVersion(prompts.StageMatchingReason, prompts.UserSubject(id))
		if _, ok := users[version]; !ok {
			users[version] = id
		}
	}
	tests := []struct {
		version    string
		wantSystem string
	}{
		{version: "v3", wantSystem: "候補のマッチング理由プロンプト"},
		{version: "v2", wantSystem: prompts.MatchingReasonSystemPrompt},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			var requestBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				requestBody = string(raw)
				w.Header().Set("Content-Type", "application/json")
				// usage が無いと利用量が記録されないので、トークン数も返す
				_, _ = w.Write([]byte(`{"output_text":"AIの理由です","usage":{"input_tokens":10,"output_tokens":5}}`))
			}))
			defer server.Close()

			var usage openai.Usage
			client := openai.NewWithBaseURL(server.URL, "gpt-4o-mini")
			client.OnUsage = func(u openai.Usage) { usage = u }
			svc := &MatchingService{aiClient: client}
			match := &entity.UserCompanyMatch{
				UserID:     users[tt.version],
				MatchScore: 80,
				Company:    models.Company{ID: 1, Name: "テスト株式会社", Industry: "IT", MainBusiness: "開発"},
			}

			// 実行主体の無い context（ワーカー・バッチと同じ）
			reason, err := svc.GenerateMatchReason(context.Background(), match, nil)
			if err != nil || reason != "AIの理由です" {
				t.Fatalf("reason=%q err=%v", reason, err)
			}
			wantSystem := strings.ReplaceAll(tt.wantSystem, "\n", `\n`)
			if !strings.Contains(requestBody, wantSystem) {
				t.Fatalf("本文が %s の版ではない: %s", tt.version, requestBody)
			}
			if match.MatchReasonPromptVersion != tt.version || usage.PromptVersion != tt.version || usage.PromptABGroup != tt.version {
				t.Fatalf("match=%q usage version=%q group=%q, want %q",
					match.MatchReasonPromptVersion, usage.PromptVersion, usage.PromptABGroup, tt.version)
			}
		})
	}
}
