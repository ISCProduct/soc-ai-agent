package matching

import (
	"Backend/domain/entity"
	"Backend/internal/models"
	"context"
	"testing"
)

// flatProfile は全軸を同じ値にしたプロファイル。
func flatProfile(companyID uint, v int) *models.CompanyWeightProfile {
	return &models.CompanyWeightProfile{
		CompanyID:             companyID,
		TechnicalOrientation:  v,
		TeamworkOrientation:   v,
		LeadershipOrientation: v,
		CreativityOrientation: v,
		StabilityOrientation:  v,
		GrowthOrientation:     v,
		WorkLifeBalance:       v,
		ChallengeSeeking:      v,
		DetailOrientation:     v,
		CommunicationSkill:    v,
	}
}

func scoresFor(v int) []entity.UserWeightScore {
	categories := []string{
		"技術志向", "チームワーク志向", "リーダーシップ志向", "創造性志向", "安定志向",
		"成長志向", "ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力",
	}
	out := make([]entity.UserWeightScore, 0, len(categories))
	for _, c := range categories {
		out = append(out, entity.UserWeightScore{WeightCategory: c, Score: v})
	}
	return out
}

// TestCalculateMatching_CompaniesWithoutProfileAreExcluded は
// プロファイルを持たない企業がマッチング結果に入らないことを固定する（#1380）。
//
// 修正前は全軸50のデフォルトで埋めていたため、スコアが50付近の
// 平均的な学生に対してマッチ度100となり、最良マッチとして上位に出ていた。
func TestCalculateMatching_CompaniesWithoutProfileAreExcluded(t *testing.T) {
	tests := []struct {
		name          string
		userScores    []entity.UserWeightScore
		profiles      map[uint]*models.CompanyWeightProfile
		wantCompanies []uint
	}{
		{
			// 修正前はこのケースで company=3（プロファイル無し）が
			// 全軸100点となり1位に出ていた
			name:       "平均的な学生: プロファイル無しの企業は結果に出ない",
			userScores: scoresFor(50),
			profiles: map[uint]*models.CompanyWeightProfile{
				1: flatProfile(1, 80),
				2: flatProfile(2, 30),
			},
			wantCompanies: []uint{1, 2},
		},
		{
			name:       "偏った学生: プロファイル無しの企業は結果に出ない",
			userScores: scoresFor(90),
			profiles: map[uint]*models.CompanyWeightProfile{
				1: flatProfile(1, 80),
				2: flatProfile(2, 30),
			},
			wantCompanies: []uint{1, 2},
		},
		{
			name:       "スコアが1軸も無い学生でも扱いは同じ",
			userScores: []entity.UserWeightScore{{WeightCategory: "技術志向", Score: 50}},
			profiles: map[uint]*models.CompanyWeightProfile{
				2: flatProfile(2, 30),
			},
			wantCompanies: []uint{2},
		},
		{
			name:          "全社プロファイル無しなら結果は0件",
			userScores:    scoresFor(50),
			profiles:      map[uint]*models.CompanyWeightProfile{},
			wantCompanies: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			companyRepo := &matchingCompanyRepo{
				companies: []models.Company{
					{ID: 1, Name: "A社"},
					{ID: 2, Name: "B社"},
					{ID: 3, Name: "プロファイル無し社"},
				},
				profiles: tt.profiles,
			}
			matchRepo := &matchingMatchRepo{}
			svc := NewMatchingService(&matchingScoreRepo{scores: tt.userScores}, companyRepo, matchRepo, nil)

			if err := svc.CalculateMatching(context.Background(), 1, "sess"); err != nil {
				t.Fatalf("CalculateMatching: %v", err)
			}

			got := make([]uint, 0, len(matchRepo.savedMatches))
			for _, m := range matchRepo.savedMatches {
				got = append(got, m.CompanyID)
				if m.CompanyID == 3 {
					t.Errorf("プロファイル無しの企業が保存された: score=%.1f", m.MatchScore)
				}
			}
			if len(got) != len(tt.wantCompanies) {
				t.Fatalf("saved companies=%v want %v", got, tt.wantCompanies)
			}
			for i := range got {
				if got[i] != tt.wantCompanies[i] {
					t.Fatalf("saved companies=%v want %v", got, tt.wantCompanies)
				}
			}
		})
	}
}

// TestGetDiagnostics_ReportsCompaniesWithoutProfile は
// 根拠の無い企業の数がログ以外の集計可能な形で取れることを固定する（#1380）。
func TestGetDiagnostics_ReportsCompaniesWithoutProfile(t *testing.T) {
	companyRepo := &matchingCompanyRepo{
		companies: []models.Company{{ID: 1}, {ID: 2}, {ID: 3}},
		profiles:  map[uint]*models.CompanyWeightProfile{1: flatProfile(1, 60)},
	}
	svc := NewMatchingService(&matchingScoreRepo{scores: scoresFor(50)}, companyRepo, nil, nil)

	diag, err := svc.GetDiagnostics(1, "sess")
	if err != nil {
		t.Fatalf("GetDiagnostics: %v", err)
	}
	if diag.CompaniesWithoutProfile != 2 {
		t.Fatalf("CompaniesWithoutProfile=%d want 2", diag.CompaniesWithoutProfile)
	}
	if diag.ActiveCompanyCount != 3 {
		t.Fatalf("ActiveCompanyCount=%d want 3", diag.ActiveCompanyCount)
	}
}
