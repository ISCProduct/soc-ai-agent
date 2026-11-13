package matching

import (
	"Backend/internal/models"
	"reflect"
	"testing"
)

// TestMinMatchedAxisCount は根拠軸数の最小値の取り方を見る。
// 呼び出し側で表示対象の match だけに絞ってから渡す前提。
func TestMinMatchedAxisCount(t *testing.T) {
	tests := []struct {
		name    string
		matches []*models.UserCompanyMatch
		want    int
	}{
		{name: "空なら0", want: 0},
		{
			name: "最小値を返す",
			matches: []*models.UserCompanyMatch{
				{MatchedAxisCount: 6},
				{MatchedAxisCount: 2},
				{MatchedAxisCount: 4},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MinMatchedAxisCount(tt.matches); got != tt.want {
				t.Fatalf("MinMatchedAxisCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestIsRecommendationProvisional_MinAxes は根拠軸数が薄いときに暫定表示になるかを見る。
func TestIsRecommendationProvisional_MinAxes(t *testing.T) {
	const enoughCategories = 10

	if IsRecommendationProvisional(enoughCategories, 90, nil, 6) {
		t.Fatal("根拠軸が十分なら暫定にしない")
	}
	if !IsRecommendationProvisional(enoughCategories, 90, nil, 2) {
		t.Fatal("根拠軸が薄いなら暫定にする")
	}
	if IsRecommendationProvisional(enoughCategories, 90, nil, 0) {
		t.Fatal("マッチが無い(0)ときは軸数を理由に暫定化しない")
	}
}

func TestIsRecommendationProvisional_EvaluatedCategories(t *testing.T) {
	if !IsRecommendationProvisional(MinEvaluatedCategoriesForFinal-1, 90, nil, 0) {
		t.Fatal("評価済みカテゴリが閾値未満なら暫定にする")
	}
	if IsRecommendationProvisional(MinEvaluatedCategoriesForFinal, 90, nil, 0) {
		t.Fatal("評価済みカテゴリが閾値ちょうどなら暫定にしない")
	}
}

func TestCountEvaluatedCategories(t *testing.T) {
	scores := []models.UserWeightScore{{Score: 3}, {Score: 0}, {Score: -1}, {Score: 5}}
	if got := CountEvaluatedCategories(scores); got != 3 {
		t.Fatalf("CountEvaluatedCategories() = %d, want 3（0だけ数えない）", got)
	}
}

// TestEmptyRecommendationReason は0件のときの理由コードの優先順位を固定する。
func TestEmptyRecommendationReason(t *testing.T) {
	tests := []struct {
		name string
		d    *MatchingDiagnostics
		want string
	}{
		{"診断が取れない", nil, "matching_results_empty"},
		{"ユーザースコア無し", &MatchingDiagnostics{UserScoreCount: 0, ActiveCompanyCount: 5}, "insufficient_user_scores"},
		{"公開企業が無い", &MatchingDiagnostics{UserScoreCount: 3, ActiveCompanyCount: 0}, "insufficient_company_data"},
		{"プロファイルが1社も無い", &MatchingDiagnostics{UserScoreCount: 3, ActiveCompanyCount: 5, CompaniesWithoutProfile: 5}, "insufficient_company_profiles"},
		{"原因が特定できない", &MatchingDiagnostics{UserScoreCount: 3, ActiveCompanyCount: 5, CompaniesWithoutProfile: 4}, "matching_results_empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EmptyRecommendationReason(tt.d); got != tt.want {
				t.Fatalf("EmptyRecommendationReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitTechStack(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"空は空配列", "", []string{}},
		{"JSON配列", `["Go","React"]`, []string{"Go", "React"}},
		{"カンマ区切り", "Go, React ,, Python", []string{"Go", "React", "Python"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitTechStack(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SplitTechStack(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDisplayableMatches_DropsUnresolvedCompany は企業を解決できない match が
// 表示にも根拠軸数の集計にも入らないことを見る。
func TestDisplayableMatches_DropsUnresolvedCompany(t *testing.T) {
	matches := []*models.UserCompanyMatch{
		{MatchedAxisCount: 1}, // Company 無し
		{MatchedAxisCount: 8, Company: models.Company{ID: 7}},
		{MatchedAxisCount: 2, Company: models.Company{}}, // ID 0
	}
	got := DisplayableMatches(matches)
	if len(got) != 1 || got[0].Company.ID != 7 {
		t.Fatalf("DisplayableMatches() = %+v, want 企業7の1件だけ", got)
	}
	if MinMatchedAxisCount(got) != 8 {
		t.Fatal("解決できない match の軸数が集計に混ざっている")
	}
	if BuildRecommendations(nil, nil) != nil {
		t.Fatal("0件は nil（JSON で null）のまま。挙動を変えない")
	}
}
