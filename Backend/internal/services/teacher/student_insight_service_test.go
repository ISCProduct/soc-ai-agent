package teacher

import (
	"errors"
	"fmt"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/repositories"
)

type stubLister struct {
	students []entity.User
	total    int64
	err      error
	// 呼び出し引数の記録（ページングと担当校が渡ることの検証用）
	gotLimit, gotOffset int
	gotQuery            string
	gotSchoolID         *uint
}

func (s *stubLister) ListStudentsPaged(limit, offset int, query string, schoolID *uint) ([]entity.User, int64, error) {
	s.gotLimit, s.gotOffset, s.gotQuery, s.gotSchoolID = limit, offset, query, schoolID
	return s.students, s.total, s.err
}

type stubScores struct {
	scores map[uint]map[string]float64
	err    error
	calls  int
	gotIDs []uint
}

func (s *stubScores) FindLatestScoresByUsers(userIDs []uint) (map[uint]map[string]float64, error) {
	s.calls++
	s.gotIDs = userIDs
	return s.scores, s.err
}

type stubIndustries struct {
	items []repositories.IndustryOption
	err   error
	calls int
}

func (s *stubIndustries) ListActive() ([]repositories.IndustryOption, error) {
	s.calls++
	return s.items, s.err
}

type stubProfiles struct {
	items []models.IndustryWeightProfile
	err   error
	calls int
}

func (s *stubProfiles) ListAll() ([]models.IndustryWeightProfile, error) {
	s.calls++
	return s.items, s.err
}

func students(n int) []entity.User {
	out := make([]entity.User, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, entity.User{ID: uint(i), Name: fmt.Sprintf("生徒%d", i), Email: fmt.Sprintf("s%d@example.com", i)})
	}
	return out
}

// TestListTendencies_NoNPlusOne は生徒数によらず一括取得が1回ずつであることを検証する。
//
// PRD 非機能要件「生徒データの集計クエリが生徒数に対してN+1にならないこと」。
// 生徒ごとに FindLatestByUser をループすると、既存の
// admin_dashboard_controller と同じ N+1 を再生産してしまう。
func TestListTendencies_NoNPlusOne(t *testing.T) {
	for _, n := range []int{1, 25, 100} {
		t.Run(fmt.Sprintf("生徒%d人", n), func(t *testing.T) {
			lister := &stubLister{students: students(n), total: int64(n)}
			scores := &stubScores{scores: map[uint]map[string]float64{}}
			inds := &stubIndustries{items: []repositories.IndustryOption{{ID: 1, Name: "情報通信業"}}}
			profs := &stubProfiles{}

			svc := NewStudentInsightService(lister, scores, inds, profs)
			got, err := svc.ListTendencies(100, 0, "", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got.Students) != n {
				t.Fatalf("件数 = %d, want %d", len(got.Students), n)
			}
			// 生徒数に関わらず1回ずつ。
			if scores.calls != 1 {
				t.Errorf("スコア取得が %d 回。生徒数に比例している(N+1)", scores.calls)
			}
			if inds.calls != 1 {
				t.Errorf("業界取得が %d 回", inds.calls)
			}
			if profs.calls != 1 {
				t.Errorf("業界プロファイル取得が %d 回", profs.calls)
			}
			if len(scores.gotIDs) != n {
				t.Errorf("一括取得に渡したID数 = %d, want %d", len(scores.gotIDs), n)
			}
		})
	}
}

// PRD 境界値: 担当生徒0人でも空一覧が返りエラーにならない。
func TestListTendencies_NoStudents(t *testing.T) {
	lister := &stubLister{students: nil, total: 0}
	scores := &stubScores{}
	inds := &stubIndustries{}
	profs := &stubProfiles{}

	svc := NewStudentInsightService(lister, scores, inds, profs)
	got, err := svc.ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Students) != 0 || got.Total != 0 {
		t.Errorf("空一覧でないと困る: %+v", got)
	}
	// 生徒がいなければ後続の取得も走らせない。
	if scores.calls != 0 || inds.calls != 0 || profs.calls != 0 {
		t.Errorf("生徒0人なのに後続クエリが走っている: scores=%d industries=%d profiles=%d",
			scores.calls, inds.calls, profs.calls)
	}
}

// 担当校の絞り込みがそのままリポジトリへ渡ること。
// ここが落ちると他校の生徒が見えるため、認可の要。
func TestListTendencies_PassesSchoolScope(t *testing.T) {
	schoolID := uint(7)
	lister := &stubLister{students: students(1), total: 1}
	svc := NewStudentInsightService(lister, &stubScores{}, &stubIndustries{}, &stubProfiles{})

	if _, err := svc.ListTendencies(10, 20, "山田", &schoolID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lister.gotSchoolID == nil || *lister.gotSchoolID != 7 {
		t.Errorf("担当校が渡っていない: %v", lister.gotSchoolID)
	}
	if lister.gotLimit != 10 || lister.gotOffset != 20 || lister.gotQuery != "山田" {
		t.Errorf("ページング/検索語が渡っていない: limit=%d offset=%d q=%q",
			lister.gotLimit, lister.gotOffset, lister.gotQuery)
	}
}

func TestListTendencies_PropagatesErrors(t *testing.T) {
	wantErr := errors.New("db down")
	tests := []struct {
		name string
		svc  *StudentInsightService
	}{
		{name: "生徒一覧", svc: NewStudentInsightService(
			&stubLister{err: wantErr}, &stubScores{}, &stubIndustries{}, &stubProfiles{})},
		{name: "スコア", svc: NewStudentInsightService(
			&stubLister{students: students(1), total: 1}, &stubScores{err: wantErr}, &stubIndustries{}, &stubProfiles{})},
		{name: "業界", svc: NewStudentInsightService(
			&stubLister{students: students(1), total: 1}, &stubScores{}, &stubIndustries{err: wantErr}, &stubProfiles{})},
		{name: "業界プロファイル", svc: NewStudentInsightService(
			&stubLister{students: students(1), total: 1}, &stubScores{}, &stubIndustries{}, &stubProfiles{err: wantErr})},
	}
	for _, tt := range tests {
		t.Run(tt.name+"のエラーを握り潰さない", func(t *testing.T) {
			if _, err := tt.svc.ListTendencies(25, 0, "", nil); !errors.Is(err, wantErr) {
				t.Errorf("err = %v, want %v", err, wantErr)
			}
		})
	}
}

// スコアがある生徒と無い生徒が混在しても、無い方だけデータ不足になること。
func TestListTendencies_MixedDataAvailability(t *testing.T) {
	lister := &stubLister{students: students(2), total: 2}
	scores := &stubScores{scores: map[uint]map[string]float64{
		1: {"技術志向": 90},
		// 生徒2 はスコアなし
	}}
	inds := &stubIndustries{items: []repositories.IndustryOption{{ID: 1, Name: "情報通信業"}}}
	svc := NewStudentInsightService(lister, scores, inds, &stubProfiles{})

	got, err := svc.ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Students[0].DataAvailable {
		t.Error("スコアがある生徒がデータ不足になっている")
	}
	if got.Students[1].DataAvailable {
		t.Error("スコアが無い生徒がデータありになっている")
	}
	if got.Students[1].TypeLabel != "分析データ不足" {
		t.Errorf("TypeLabel = %q", got.Students[1].TypeLabel)
	}
}
