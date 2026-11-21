package matching

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// AI の理由文が長すぎる・入力に無い数字を書いたときは、テンプレートの理由に戻す（SOCAIAGENT-702）。
func TestGenerateMatchReason_出力を検証してテンプレートへ戻す(t *testing.T) {
	t.Setenv("MATCHING_REASON_USE_AI", "true")
	ok := "技術志向が強く、Webサービス開発に挑戦する文化と相性が良さそうです。総合マッチ度は82.5で、成長の機会も期待できます。"
	tests := []struct {
		name         string
		aiText       string
		wantTemplate bool
	}{
		{name: "入力の範囲の文はそのまま使う", aiText: ok},
		{name: "全角数字も入力と照らして通す", aiText: "総合マッチ度は８２.５で、技術志向が活かせそうです。"},
		{name: "長すぎる文はテンプレートへ", aiText: strings.Repeat("あ", matchReasonMaxRunes+1), wantTemplate: true},
		{name: "入力に無い数字（年収など）はテンプレートへ", aiText: "年収600万円が見込め、技術志向が活かせます。", wantTemplate: true},
		{name: "入力に無い全角数字もテンプレートへ", aiText: "従業員３０００名の大企業です。", wantTemplate: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"output_text": tt.aiText})
			}))
			defer srv.Close()
			svc := &MatchingService{aiClient: openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")}
			match := &models.UserCompanyMatch{
				UserID: 1, MatchScore: 82.5, TechnicalMatch: 90,
				Company: models.Company{ID: 1, Name: "テスト株式会社", Industry: "IT", MainBusiness: "Webサービス開発"},
			}
			scores := []models.UserWeightScore{{WeightCategory: "技術志向", Score: 88}}

			got, err := svc.GenerateMatchReason(context.Background(), match, scores)
			if err != nil {
				t.Fatal(err)
			}
			template := BuildMatchReason(match, scores)
			if tt.wantTemplate {
				if got != template {
					t.Errorf("テンプレートに戻っていない: %q", got)
				}
				if match.MatchReasonPromptVersion != "" {
					t.Errorf("テンプレートなのに版が付いた: %q", match.MatchReasonPromptVersion)
				}
			} else if got != strings.TrimSpace(tt.aiText) {
				t.Errorf("AI の文が使われていない: %q", got)
			}
		})
	}
}
