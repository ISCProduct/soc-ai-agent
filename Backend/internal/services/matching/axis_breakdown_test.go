package matching

import (
	"errors"
	"slices"
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
	// スコアは整数に丸めて出す。一致度は推薦と同じ式（100 - 差）で、丸める前のスコアから計算する
	// （推薦の一覧と同じ。丸めてから差を取ると ±1 ずれる）
	if *tech.UserScore != 72 || tech.CompanyWeight != 80 || *tech.Match != CalculateCategoryMatch(72.4, 80) {
		t.Fatalf("technical = %+v", tech)
	}
	// 表示名は推薦の一覧の「噛み合った軸」と同じ
	if tech.Label != "技術力" || out.Axes[7].Label != "挑戦意欲" || out.Axes[8].Label != "緻密さ" {
		t.Fatalf("labels = %+v", out.Axes)
	}
	// 診断で測れていない軸は数値を出さない（0点と区別する）
	if growth := out.Axes[5]; growth.UserScore != nil || growth.Match != nil || growth.CompanyWeight != 70 {
		t.Fatalf("growth = %+v", growth)
	}
	// 企業が重視しているのに測れていない軸は、名前を挙げる
	if len(out.Unmeasured) != 1 || out.Unmeasured[0] != "成長意欲" {
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
	if !strings.Contains(out.Gaps[0].Text, "「挑戦意欲」を重視しています（80）") || !strings.Contains(out.Gaps[0].Text, "今のスコアは 10") {
		t.Fatalf("text = %s", out.Gaps[0].Text)
	}
}

func TestBuildAxisBreakdown_スコアが無くても空の配列(t *testing.T) {
	out := BuildAxisBreakdown(map[string]float64{}, profileWith(func(*models.CompanyWeightProfile) {}))
	if out.Strengths == nil || out.Gaps == nil || out.Unmeasured == nil || len(out.Axes) != 10 {
		t.Fatalf("out = %+v", out)
	}
}

// 丸める順番で一覧と内訳の一致度がずれない（スコア 72.5・重視度 60 → 一覧も内訳も 87.5 で、表示は 88）。
func TestBuildAxisBreakdown_一致度は丸める前のスコアで計算する(t *testing.T) {
	out := BuildAxisBreakdown(map[string]float64{"技術志向": 72.5},
		profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = 60 }))
	if got := *out.Axes[0].Match; got != 87.5 {
		t.Fatalf("match = %v, want 87.5", got)
	}
}

// 範囲外のスコアは 0〜100 に収めてから、表・文に出す。
func TestBuildAxisBreakdown_範囲外のスコアを収める(t *testing.T) {
	out := BuildAxisBreakdown(map[string]float64{"技術志向": -10, "チャレンジ志向": 130},
		profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = 80 }))
	if *out.Axes[0].UserScore != 0 || *out.Axes[7].UserScore != 100 {
		t.Fatalf("axes = %+v", out.Axes)
	}
	if mine := BuildMyAxisScores(map[string]float64{"技術志向": -10}); *mine[0].Score != 0 {
		t.Fatalf("mine = %+v", mine[0])
	}
}

// 引数を記録するスタブ。「本人の推薦・本人の診断だけを使う」ことを、渡した値で確かめる。
type recMatchRepo struct {
	*stubToggleFavoriteMatchRepo
	gotID uint
}

func (r *recMatchRepo) FindByID(id uint) (*models.UserCompanyMatch, error) {
	r.gotID = id
	return r.stubToggleFavoriteMatchRepo.FindByID(id)
}

type recCompanyRepo struct {
	*matchingCompanyRepo
	gotIDs []uint
	err    error
}

func (r *recCompanyRepo) GetWeightProfilesByCompanyIDs(ids []uint) (map[uint]*models.CompanyWeightProfile, error) {
	r.gotIDs = ids
	if r.err != nil {
		return nil, r.err
	}
	return r.profiles, nil
}

type recScoreRepo struct {
	*matchingScoreRepo
	gotUser    uint
	gotSession string
	err        error
}

func (r *recScoreRepo) FindByUserAndSession(userID uint, sessionID string) ([]models.UserWeightScore, error) {
	r.gotUser, r.gotSession = userID, sessionID
	if r.err != nil {
		return nil, r.err
	}
	return r.scores, nil
}

type recAssessment struct {
	scores  map[string]float64
	gotUser uint
}

func (r *recAssessment) LatestStandardScores(userID uint) (map[string]float64, error) {
	r.gotUser = userID
	return r.scores, nil
}

type axisFixture struct {
	svc        *MatchingService
	matches    *recMatchRepo
	companies  *recCompanyRepo
	scores     *recScoreRepo
	assessment *recAssessment
}

func newAxisFixture(match *models.UserCompanyMatch, findErr error) axisFixture {
	f := axisFixture{
		matches: &recMatchRepo{stubToggleFavoriteMatchRepo: &stubToggleFavoriteMatchRepo{findByIDResult: match, findByIDErr: findErr}},
		companies: &recCompanyRepo{matchingCompanyRepo: &matchingCompanyRepo{profiles: map[uint]*models.CompanyWeightProfile{
			1: profileWith(func(p *models.CompanyWeightProfile) { p.TechnicalOrientation = 80 }),
		}}},
		scores: &recScoreRepo{matchingScoreRepo: &matchingScoreRepo{scores: []models.UserWeightScore{
			{WeightCategory: "技術志向", Score: 40}, {WeightCategory: "安定志向", Score: 61},
		}}},
		assessment: &recAssessment{scores: map[string]float64{"技術志向": 81, "成長志向": 90}},
	}
	f.svc = &MatchingService{matchRepo: f.matches, companyRepo: f.companies, userWeightScoreRepo: f.scores}
	f.svc.SetAssessmentScoreReader(f.assessment)
	return f
}

func axisMatch() *models.UserCompanyMatch {
	return &models.UserCompanyMatch{ID: 5, UserID: 7, SessionID: "s-match", CompanyID: 1, Company: models.Company{ID: 1, Name: "A社"}}
}

func TestGetAxisBreakdown(t *testing.T) {
	t.Setenv("ASSESSMENT_SCORE_MERGE", "")
	f := newAxisFixture(axisMatch(), nil)

	out, err := f.svc.GetAxisBreakdown(5, 7)
	if err != nil || out.MatchID != 5 || out.CompanyName != "A社" {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	// 推薦は matchID で引き、プロファイルは推薦の企業、スコアは本人の・推薦の診断のものを使う
	if f.matches.gotID != 5 || !slices.Equal(f.companies.gotIDs, []uint{1}) ||
		f.scores.gotUser != 7 || f.scores.gotSession != "s-match" || f.assessment.gotUser != 7 {
		t.Fatalf("引数: match=%d companies=%v scores=(%d,%q) assessment=%d",
			f.matches.gotID, f.companies.gotIDs, f.scores.gotUser, f.scores.gotSession, f.assessment.gotUser)
	}
	// 技術: チャット40と検査81 → 60.5 → 丸めて61
	if got := *out.Axes[0].UserScore; got != 61 {
		t.Fatalf("technical = %v, want 61", got)
	}

	// 他人の推薦・存在しない推薦・プロファイルの無い企業・削除された企業は、どれも見つからない扱い
	other := newAxisFixture(axisMatch(), nil)
	if _, err := other.svc.GetAxisBreakdown(5, 8); !errors.Is(err, shared.ErrNotFound) {
		t.Errorf("他人の推薦: err = %v", err)
	}
	if _, err := newAxisFixture(nil, gorm.ErrRecordNotFound).svc.GetAxisBreakdown(5, 7); !errors.Is(err, shared.ErrNotFound) {
		t.Errorf("存在しない推薦: err = %v", err)
	}
	noProfile := newAxisFixture(axisMatch(), nil)
	noProfile.companies.profiles = map[uint]*models.CompanyWeightProfile{}
	if _, err := noProfile.svc.GetAxisBreakdown(5, 7); !errors.Is(err, shared.ErrNotFound) {
		t.Errorf("プロファイルが無い: err = %v", err)
	}
	deleted := axisMatch()
	deleted.Company = models.Company{}
	if _, err := newAxisFixture(deleted, nil).svc.GetAxisBreakdown(5, 7); !errors.Is(err, shared.ErrNotFound) {
		t.Errorf("削除された企業: err = %v", err)
	}
}

// Repository の失敗は、元のエラーのまま返す（404「見られません」に化けさせない）。
func TestGetAxisBreakdown_保存先のエラーはそのまま返す(t *testing.T) {
	boom := errors.New("db down")
	cases := map[string]func(f axisFixture){
		"推薦":     func(f axisFixture) { f.matches.findByIDErr = boom },
		"プロファイル": func(f axisFixture) { f.companies.err = boom },
		"スコア":    func(f axisFixture) { f.scores.err = boom },
	}
	for name, breakIt := range cases {
		f := newAxisFixture(axisMatch(), nil)
		breakIt(f)
		if _, err := f.svc.GetAxisBreakdown(5, 7); !errors.Is(err, boom) || errors.Is(err, shared.ErrNotFound) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// 「あなたの10の傾向」と「軸ごとの内訳」は、適性検査を合流させた同じ値を出す（同じ画面で食い違わない）。
func TestGetMyAxisScores_内訳と同じ合流後の値(t *testing.T) {
	t.Setenv("ASSESSMENT_SCORE_MERGE", "")
	f := newAxisFixture(axisMatch(), nil)

	mine, err := f.svc.GetMyAxisScores(7, "s1")
	if err != nil || len(mine) != len(axes) {
		t.Fatalf("mine = %+v err = %v", mine, err)
	}
	// 本人の・指定した診断のスコアを使う
	if f.scores.gotUser != 7 || f.scores.gotSession != "s1" || f.assessment.gotUser != 7 {
		t.Fatalf("引数: scores=(%d,%q) assessment=%d", f.scores.gotUser, f.scores.gotSession, f.assessment.gotUser)
	}
	// 技術: チャット40と検査81 → 60.5 → 丸めて61。成長: 検査だけ90。安定: チャットだけ61
	want := map[string]float64{"technical": 61, "growth": 90, "stability": 61}
	for _, m := range mine {
		if w, ok := want[m.Key]; ok && (m.Score == nil || *m.Score != w) {
			t.Fatalf("%s = %v, want %v", m.Key, m.Score, w)
		}
	}
	breakdown, err := f.svc.GetAxisBreakdown(5, 7)
	if err != nil {
		t.Fatal(err)
	}
	f.scores.gotSession = ""
	for i, a := range axes {
		got, w := mine[i].Score, breakdown.Axes[i].UserScore
		if (got == nil) != (w == nil) || (got != nil && *got != *w) {
			t.Fatalf("%s: 傾向=%v 内訳=%v", a.category, got, w)
		}
		if mine[i].Label != breakdown.Axes[i].Label {
			t.Fatalf("%s: 表示名が違う 傾向=%s 内訳=%s", a.key, mine[i].Label, breakdown.Axes[i].Label)
		}
	}
	// 測れていない軸は null（0点と区別する）
	if mine[1].Label != "チームワーク" || mine[1].Score != nil {
		t.Fatalf("teamwork = %+v", mine[1])
	}
}

// チャットのスコアが無い診断では、適性検査だけを合流させない（推薦の計算と同じ。推薦0件の画面と食い違わない）。
func TestGetMyAxisScores_チャットのスコアが無ければ合流しない(t *testing.T) {
	t.Setenv("ASSESSMENT_SCORE_MERGE", "")
	f := newAxisFixture(axisMatch(), nil)
	f.scores.scores = nil

	mine, err := f.svc.GetMyAxisScores(7, "s1")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mine {
		if m.Score != nil {
			t.Fatalf("%s = %v（適性検査だけの値を出している）", m.Key, *m.Score)
		}
	}
}

func TestAxes_10軸の定義はひとつ(t *testing.T) {
	if len(axes) != 10 {
		t.Fatalf("axes = %d", len(axes))
	}
	p := profileWith(func(p *models.CompanyWeightProfile) { p.DetailOrientation = 77 })
	for i, a := range axes {
		if axisNames[i] != a.category || profileAxisValues(p)[i] != float64(a.weight(p)) {
			t.Fatalf("%s: axisNames / profileAxisValues が axes とずれている", a.key)
		}
	}
}
