package matching

import (
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"

	"gorm.io/gorm"
)

// SOCAIAGENT-611: 推薦1件の軸別のマッチの内訳。

func profileWith(f func(p *models.CompanyWeightProfile)) *models.CompanyWeightProfile {
	p := &models.CompanyWeightProfile{
		TechnicalOrientation: 50, TeamworkOrientation: 50, LeadershipOrientation: 50, CreativityOrientation: 50,
		StabilityOrientation: 50, GrowthOrientation: 50, WorkLifeBalance: 50, ChallengeSeeking: 50,
		DetailOrientation: 50, CommunicationSkill: 50,
	}
	f(p)
	return p
}

func keys(notes []AxisNote) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.Key)
	}
	return out
}

func TestBuildAxisBreakdown_10軸の数値と未計測(t *testing.T) {
	out := BuildAxisBreakdown(
		map[string]float64{"技術志向": 72.4, "安定志向": 30},
		profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = 80; p.GrowthOrientation = 70 }),
	)
	if len(out.Axes) != 10 || out.Axes[0].Key != "technical" || out.Axes[9].Key != "communication" {
		t.Fatalf("axes = %+v", out.Axes)
	}
	tech := out.Axes[0]
	// スコアは整数に丸め、一致度は推薦と同じ式（100 - 差）
	if *tech.UserScore != 72 || tech.CompanyWeight != 80 || *tech.Match != CalculateCategoryMatch(72, 80) {
		t.Fatalf("technical = %+v", tech)
	}
	// 診断で測れていない軸は数値を出さない（0点と区別する）
	if growth := out.Axes[5]; growth.UserScore != nil || growth.Match != nil || growth.CompanyWeight != 70 {
		t.Fatalf("growth = %+v", growth)
	}
	// 企業が重視しているのに測れていない軸は、名前を挙げる
	if len(out.Unmeasured) != 1 || out.Unmeasured[0] != "成長志向" {
		t.Fatalf("unmeasured = %v", out.Unmeasured)
	}
}

// 強み: あなたも企業も 60 以上で、差が 15 以内。境界の 60 と 15 は含む。
func TestBuildAxisBreakdown_強みの境界(t *testing.T) {
	for _, tt := range []struct {
		name         string
		user, weight float64
		want         bool
	}{
		{"両方60で差0", 60, 60, true},
		{"差15", 75, 60, true},
		{"差16", 76, 60, false},
		{"あなたが59", 59, 60, false},
		{"企業が59", 70, 59, false},
	} {
		out := BuildAxisBreakdown(map[string]float64{"技術志向": tt.user},
			profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = int(tt.weight) }))
		if got := len(out.Strengths) == 1; got != tt.want {
			t.Errorf("%s: strengths = %v", tt.name, out.Strengths)
		}
	}
}

// 補強ポイント: 企業が 60 以上で、あなたが 20 以上下回る。上回っているときは補強ポイントにしない。
func TestBuildAxisBreakdown_補強ポイントの境界(t *testing.T) {
	for _, tt := range []struct {
		name         string
		user, weight float64
		want         bool
	}{
		{"差20", 60, 80, true},
		{"差19", 61, 80, false},
		{"企業が59", 30, 59, false},
		{"あなたが上回る", 100, 60, false},
	} {
		out := BuildAxisBreakdown(map[string]float64{"技術志向": tt.user},
			profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = int(tt.weight) }))
		if got := len(out.Gaps) == 1; got != tt.want {
			t.Errorf("%s: gaps = %v", tt.name, out.Gaps)
		}
	}
}

func TestBuildAxisBreakdown_並びと件数の上限(t *testing.T) {
	scores := map[string]float64{
		"技術志向": 80, "チームワーク志向": 70, "リーダーシップ志向": 65, "創造性志向": 61, // 強みの候補4件
		"安定志向": 20, "成長志向": 40, "ワークライフバランス": 50, "チャレンジ志向": 10, // 補強ポイントの候補4件
	}
	p := profileWith(func(p *models.CompanyWeightProfile) {
		p.TechnicalOrientation, p.TeamworkOrientation, p.LeadershipOrientation, p.CreativityOrientation = 80, 75, 75, 75
		p.StabilityOrientation, p.GrowthOrientation, p.WorkLifeBalance, p.ChallengeSeeking = 80, 80, 80, 80
	})
	out := BuildAxisBreakdown(scores, p)
	// 強みは一致度の高い順（技術100、チーム95、リーダー90、創造性86）で3件まで
	if got := strings.Join(keys(out.Strengths), ","); got != "technical,teamwork,leadership" {
		t.Fatalf("strengths = %s", got)
	}
	// 補強ポイントは差の大きい順（チャレンジ70、安定60、成長40、WLB30）で3件まで
	if got := strings.Join(keys(out.Gaps), ","); got != "challenge,stability,growth" {
		t.Fatalf("gaps = %s", got)
	}
	if !strings.Contains(out.Gaps[0].Text, "「チャレンジ志向」を重視しています（80）") || !strings.Contains(out.Gaps[0].Text, "今のスコアは 10") {
		t.Fatalf("text = %s", out.Gaps[0].Text)
	}
}

func TestBuildAxisBreakdown_スコアが無くても空の配列(t *testing.T) {
	out := BuildAxisBreakdown(map[string]float64{}, profileWith(func(*models.CompanyWeightProfile) {}))
	if out.Strengths == nil || out.Gaps == nil || out.Unmeasured == nil || len(out.Axes) != 10 {
		t.Fatalf("out = %+v", out)
	}
}

func TestGetAxisBreakdown(t *testing.T) {
	match := &models.UserCompanyMatch{ID: 5, UserID: 7, SessionID: "s1", CompanyID: 1, Company: models.Company{Name: "A社"}}
	newSvc := func(m *models.UserCompanyMatch, err error, profiles map[uint]*models.CompanyWeightProfile) *MatchingService {
		return &MatchingService{
			matchRepo:           &stubToggleFavoriteMatchRepo{findByIDResult: m, findByIDErr: err},
			companyRepo:         &matchingCompanyRepo{profiles: profiles},
			userWeightScoreRepo: &matchingScoreRepo{scores: []models.UserWeightScore{{WeightCategory: "技術志向", Score: 70}}},
		}
	}
	profiles := map[uint]*models.CompanyWeightProfile{1: profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = 80 })}

	out, err := newSvc(match, nil, profiles).GetAxisBreakdown(5, 7)
	if err != nil || out.MatchID != 5 || out.CompanyName != "A社" || *out.Axes[0].UserScore != 70 {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	// 他人の推薦・存在しない推薦・プロファイルの無い企業は、どれも見つからない扱い
	for name, svc := range map[string]*MatchingService{
		"他人の推薦":     newSvc(match, nil, profiles),
		"存在しない推薦":   newSvc(nil, gorm.ErrRecordNotFound, profiles),
		"プロファイルが無い": newSvc(match, nil, map[uint]*models.CompanyWeightProfile{}),
	} {
		userID := uint(7)
		if name == "他人の推薦" {
			userID = 8
		}
		if _, err := svc.GetAxisBreakdown(5, userID); !errors.Is(err, shared.ErrNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
