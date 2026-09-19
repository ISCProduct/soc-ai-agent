package diagnosis

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"testing"
)

func TestHeuristicFlags_FewAxesAndSaturated(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 60},
		{WeightCategory: "成長志向", Score: 40},
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 98, MatchedAxisCount: 2},
		{CompanyID: 2, MatchScore: 97, MatchedAxisCount: 2},
		{CompanyID: 3, MatchScore: 96, MatchedAxisCount: 2},
	}
	flags := heuristicFlags(scores, matches)
	want := map[string]bool{
		"few_measured_axes":   true,
		"saturated_matches":   true,
		"thin_match_evidence": true,
	}
	for _, f := range flags {
		if !want[f] {
			t.Fatalf("unexpected flag %q in %v", f, flags)
		}
		delete(want, f)
	}
	if len(want) > 0 {
		t.Fatalf("missing flags %v; got %v", want, flags)
	}
}

func TestHeuristicFlags_ZeroAxesIsThin(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 60},
		{WeightCategory: "成長志向", Score: 40},
		{WeightCategory: "細部志向", Score: 50},
		{WeightCategory: "安定志向", Score: 55},
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 0},
		{CompanyID: 2, MatchScore: 70, MatchedAxisCount: 0},
	}
	flags := heuristicFlags(scores, matches)
	found := false
	for _, f := range flags {
		if f == "thin_match_evidence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("MatchedAxisCount=0 should flag thin_match_evidence, got %v", flags)
	}
}

func TestHeuristicFlags_SingleMatch(t *testing.T) {
	scores := make([]entity.UserWeightScore, 6)
	for i := range scores {
		scores[i] = entity.UserWeightScore{WeightCategory: "c", Score: 50}
	}
	flags := heuristicFlags(scores, []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 88, MatchedAxisCount: 6},
	})
	found := false
	for _, f := range flags {
		if f == "single_match_only" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected single_match_only, got %v", flags)
	}
}

func TestHeuristicFlags_Healthy(t *testing.T) {
	scores := make([]entity.UserWeightScore, 0, 8)
	for i := 0; i < 8; i++ {
		scores = append(scores, entity.UserWeightScore{WeightCategory: "c", Score: 50 + i})
	}
	matches := []*entity.UserCompanyMatch{
		{CompanyID: 1, MatchScore: 88, MatchedAxisCount: 8},
		{CompanyID: 2, MatchScore: 70, MatchedAxisCount: 8},
	}
	flags := heuristicFlags(scores, matches)
	if len(flags) != 0 {
		t.Fatalf("expected no flags, got %v", flags)
	}
}

func TestHeuristicFlags_ScoreZeroStillCounts(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "a", Score: 0},
		{WeightCategory: "b", Score: 0},
		{WeightCategory: "c", Score: 0},
		{WeightCategory: "d", Score: 0},
	}
	flags := heuristicFlags(scores, nil)
	for _, f := range flags {
		if f == "few_measured_axes" || f == "no_measured_axes" {
			t.Fatalf("score=0 rows are measured; unexpected %q in %v", f, flags)
		}
	}
}

func TestChatEvidenceFlags_MostlyChoiceOnly(t *testing.T) {
	msgs := []models.ChatMessage{
		{Role: "user", Content: "A"},
		{Role: "user", Content: "B"},
		{Role: "user", Content: "C"},
		{Role: "assistant", Content: "次の質問"},
	}
	flags, stats := chatEvidenceFlags(msgs)
	if stats["choice_only"] != 3 {
		t.Fatalf("stats=%v", stats)
	}
	want := map[string]bool{"thin_chat_evidence": true, "mostly_choice_only": true}
	for _, f := range flags {
		if !want[f] {
			t.Fatalf("unexpected %q in %v", f, flags)
		}
		delete(want, f)
	}
	if len(want) > 0 {
		t.Fatalf("missing %v got %v", want, flags)
	}
}

func TestChatEvidenceFlags_SupportingReasonIsSubstantial(t *testing.T) {
	msgs := []models.ChatMessage{
		{Role: "user", Content: "A: チームで進めるのが好きで調整役をしてきました"},
		{Role: "user", Content: "インターンで要件定義から実装まで担当し、締切前に品質を担保しました"},
	}
	flags, stats := chatEvidenceFlags(msgs)
	if stats["with_reason"] != 1 || stats["free_text"] != 1 {
		t.Fatalf("stats=%v", stats)
	}
	for _, f := range flags {
		if f == "thin_chat_evidence" || f == "mostly_choice_only" {
			t.Fatalf("unexpected weak flag %q in %v", f, flags)
		}
	}
}

func TestHeuristicConfidence_ChatFlagsPenalizeHarder(t *testing.T) {
	scores := make([]entity.UserWeightScore, 8)
	for i := range scores {
		scores[i] = entity.UserWeightScore{WeightCategory: "c", Score: 50}
	}
	base := heuristicConfidence(scores, nil)
	weak := heuristicConfidence(scores, []string{"mostly_choice_only", "thin_chat_evidence"})
	if weak >= base-20 {
		t.Fatalf("chat flags should cut confidence hard: base=%d weak=%d", base, weak)
	}
}

// TestHeuristicFlags_ThinMatchUsesMinimum は thin_match_evidence の判定が
// 先頭ではなく上位マッチ全体の最小 MatchedAxisCount を見ることを検証する。
// 先頭だけ見ていると「1位は根拠十分、2位以降は根拠が薄い」結果を
// 信頼できる診断として扱い、confidence のペナルティも落ちる。
func TestHeuristicFlags_ThinMatchUsesMinimum(t *testing.T) {
	scores := []entity.UserWeightScore{
		{WeightCategory: "技術志向", Score: 60},
		{WeightCategory: "成長志向", Score: 40},
		{WeightCategory: "チームワーク志向", Score: 55},
		{WeightCategory: "細部志向", Score: 45},
		{WeightCategory: "チャレンジ志向", Score: 50},
		{WeightCategory: "コミュニケーション力", Score: 52},
	}

	tests := []struct {
		name     string
		matches  []*entity.UserCompanyMatch
		wantFlag bool
	}{
		{
			name: "全マッチの根拠軸が十分ならフラグなし",
			matches: []*entity.UserCompanyMatch{
				{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 6},
				{CompanyID: 2, MatchScore: 60, MatchedAxisCount: 5},
			},
			wantFlag: false,
		},
		{
			name: "先頭は十分でも後続が薄ければフラグを立てる",
			matches: []*entity.UserCompanyMatch{
				{CompanyID: 1, MatchScore: 80, MatchedAxisCount: 6},
				{CompanyID: 2, MatchScore: 60, MatchedAxisCount: 2},
			},
			wantFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := false
			for _, f := range heuristicFlags(scores, tt.matches) {
				if f == "thin_match_evidence" {
					got = true
				}
			}
			if got != tt.wantFlag {
				t.Fatalf("thin_match_evidence = %v, want %v", got, tt.wantFlag)
			}
		})
	}
}
