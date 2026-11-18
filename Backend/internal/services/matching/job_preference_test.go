package matching

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
)

type fakeJobPref struct {
	categoryID uint
	catErr     error
	hiring     map[uint]bool
	hiringErr  error
}

func (f fakeJobPref) SessionJobCategoryID(string) (uint, error) { return f.categoryID, f.catErr }
func (f fakeJobPref) CompanyIDsHiringCategory([]uint, uint) (map[uint]bool, error) {
	return f.hiring, f.hiringErr
}

// 同じプロファイルの 2 社（A: 選んだ職種の募集あり、B: なし）で、A だけが加点される。
func runJobPref(t *testing.T, pref JobPreferenceReader) map[uint]*models.UserCompanyMatch {
	t.Helper()
	profile := func(id uint) *models.CompanyWeightProfile {
		return &models.CompanyWeightProfile{CompanyID: id, TechnicalOrientation: 60, TeamworkOrientation: 60}
	}
	companyRepo := &matchingCompanyRepo{
		companies: []models.Company{{ID: 1, Name: "A社"}, {ID: 2, Name: "B社"}},
		profiles:  map[uint]*models.CompanyWeightProfile{1: profile(1), 2: profile(2)},
	}
	scoreRepo := &matchingScoreRepo{scores: []models.UserWeightScore{{WeightCategory: "技術志向", Score: 30}, {WeightCategory: "チームワーク志向", Score: 20}}}
	matchRepo := &matchingMatchRepo{}
	svc := NewMatchingService(scoreRepo, companyRepo, matchRepo, nil)
	if pref != nil {
		svc.SetJobPreferenceReader(pref)
	}
	if err := svc.CalculateMatching(context.Background(), 1, "s1"); err != nil {
		t.Fatal(err)
	}
	out := map[uint]*models.UserCompanyMatch{}
	for _, m := range matchRepo.savedMatches {
		out[m.CompanyID] = m
	}
	return out
}

func TestJobPreference_BonusOnlyForHiringCompany(t *testing.T) {
	got := runJobPref(t, fakeJobPref{categoryID: 3, hiring: map[uint]bool{1: true}})
	a, b := got[1], got[2]
	if !a.JobMatched || b.JobMatched {
		t.Fatalf("募集がある A だけが JobMatched: A=%v B=%v", a.JobMatched, b.JobMatched)
	}
	if want := b.MatchScore + JobMatchBonus; a.MatchScore != want {
		t.Fatalf("A=%.1f、B + 加点 = %.1f のはず", a.MatchScore, want)
	}
}

// 加点だけで、除外はしない。募集データが無い企業は、従来どおりの適合度で残る。
func TestJobPreference_NeverExcludes(t *testing.T) {
	base := runJobPref(t, nil)
	with := runJobPref(t, fakeJobPref{categoryID: 3, hiring: map[uint]bool{1: true}})
	if len(with) != len(base) {
		t.Fatalf("企業数が変わった: %d → %d", len(base), len(with))
	}
	if with[2].MatchScore != base[2].MatchScore {
		t.Fatalf("募集の無い企業の適合度が変わった: %.1f → %.1f", base[2].MatchScore, with[2].MatchScore)
	}
}

func TestJobPreference_NoChangeWhenUnavailable(t *testing.T) {
	base := runJobPref(t, nil)
	for name, pref := range map[string]JobPreferenceReader{
		"職種を選んでいない":  fakeJobPref{categoryID: 0, hiring: map[uint]bool{1: true}},
		"職種の取得に失敗":   fakeJobPref{catErr: errors.New("db"), hiring: map[uint]bool{1: true}},
		"募集の取得に失敗":   fakeJobPref{categoryID: 3, hiringErr: errors.New("db")},
		"募集のある企業が無い": fakeJobPref{categoryID: 3, hiring: map[uint]bool{}},
	} {
		t.Run(name, func(t *testing.T) {
			got := runJobPref(t, pref)
			for id, m := range got {
				if m.JobMatched || m.MatchScore != base[id].MatchScore {
					t.Fatalf("企業%d: 変わらないはず: %+v", id, m)
				}
			}
		})
	}
}

func TestJobPreference_CanBeSwitchedOff(t *testing.T) {
	t.Setenv("MATCHING_JOB_BONUS", "off")
	base := runJobPref(t, nil)
	got := runJobPref(t, fakeJobPref{categoryID: 3, hiring: map[uint]bool{1: true}})
	if got[1].MatchScore != base[1].MatchScore {
		t.Fatalf("off のときは加点しない: %.1f → %.1f", base[1].MatchScore, got[1].MatchScore)
	}
	if !got[1].JobMatched {
		t.Fatalf("加点しなくても、募集の有無は保存する")
	}
}

func TestApplyJobBonus_CapsAt100(t *testing.T) {
	if got := applyJobBonus(70); got != 70+JobMatchBonus {
		t.Fatalf("got %v", got)
	}
	if got := applyJobBonus(97); got != 100 {
		t.Fatalf("上限は100: %v", got)
	}
}

func TestBuildRecommendations_JobMatched(t *testing.T) {
	rec := BuildRecommendations([]*models.UserCompanyMatch{{Company: models.Company{ID: 1}, JobMatched: true}, {Company: models.Company{ID: 2}}}, nil)
	if len(rec) != 2 || !rec[0].JobMatched || rec[1].JobMatched {
		t.Fatalf("%+v", rec)
	}
}
