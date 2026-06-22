package services_test

// マッチングサービス内部ロジックのユニットテスト
// 実行: cd Backend && go test ./test/services/... -run TestFormatTopUserScores -v

import (
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/services"
)

func TestFormatTopUserScores(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "成長志向", Score: 72},
		{WeightCategory: "技術志向", Score: 90},
		{WeightCategory: "チームワーク志向", Score: 81},
		{WeightCategory: "安定志向", Score: 40},
	}

	got := services.FormatTopUserScores(scores)
	wantParts := []string{"技術志向:90", "チームワーク志向:81", "成長志向:72"}
	for _, part := range wantParts {
		if !strings.Contains(got, part) {
			t.Fatalf("expected %q in %q", part, got)
		}
	}
}

func TestGenerateMatchReasonFallbackWhenAIClientNil(t *testing.T) {
	svc := services.NewMatchingServiceForTest()
	match := &entity.UserCompanyMatch{
		MatchScore: 88.5,
		Company: &entity.Company{
			ID:           1,
			Name:         "テスト株式会社",
			Industry:     "IT",
			MainBusiness: "Webサービス開発",
			Culture:      "挑戦を歓迎する",
			WorkStyle:    "ハイブリッド",
		},
		TechnicalMatch: 95.0,
		TeamworkMatch:  90.0,
		GrowthMatch:    92.0,
	}
	userScores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 88},
		{WeightCategory: "成長志向", Score: 84},
	}

	got, err := svc.GenerateMatchReason(t.Context(), match, userScores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("reason should not be empty")
	}
	if !strings.Contains(got, "テスト株式会社") {
		t.Fatalf("expected company name in reason, got: %q", got)
	}
}
