package teacher

import (
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
)

type stubResumeFacts struct {
	byUser map[uint]models.ResumeLatestFact
	err    error
	calls  int
	gotIDs []uint
}

func (s *stubResumeFacts) FindLatestResumeFactsByUsers(userIDs []uint) (map[uint]models.ResumeLatestFact, error) {
	s.calls++
	s.gotIDs = userIDs
	return s.byUser, s.err
}

func resumeService(facts *stubResumeFacts, students ...entity.User) *StudentInsightService {
	svc := NewStudentInsightService(
		&stubLister{students: students, total: int64(len(students))},
		&stubScores{scores: map[uint]map[string]float64{}},
		&stubIndustries{},
		&stubProfiles{},
	)
	if facts != nil {
		svc.SetResumeFactReader(facts)
	}
	return svc
}

// 履歴書要対応の生徒だけに絞れること（#1030）。
func TestListTendencies_ResumeNeedsAttentionOnly(t *testing.T) {
	t.Setenv("RESUME_COMPLETENESS_THRESHOLD", "60")
	scoreLow := 40
	scoreOK := 80
	facts := &stubResumeFacts{byUser: map[uint]models.ResumeLatestFact{
		// 1: map に無い = 未提出 → 要対応
		2: {HasDocument: true, LatestScore: &scoreLow}, // スコア低
		3: {HasDocument: true, LatestScore: &scoreOK},  // OK
	}}
	// user 1 は byUser に入れない（未提出）
	res, err := resumeService(facts, threeStudents...).ListTendenciesWithFilters(25, 0, "", nil, false, true)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Students) != 2 {
		t.Fatalf("要対応は2人: %+v", res.Students)
	}
	got := map[uint]string{}
	for _, s := range res.Students {
		if s.ResumeStatus == nil || !s.ResumeStatus.NeedsAttention {
			t.Fatalf("要対応フラグが付いていない: %+v", s)
		}
		got[s.UserID] = s.ResumeStatus.Reason
	}
	if got[1] != "未提出" {
		t.Errorf("user1 reason = %q, want 未提出", got[1])
	}
	if got[2] != "スコア低" {
		t.Errorf("user2 reason = %q, want スコア低", got[2])
	}
	if res.Total != 2 {
		t.Errorf("Total = %d, want 2", res.Total)
	}
}

// 絞り込みなしでも履歴書ステータスは付く。
func TestListTendencies_AttachesResumeStatusWithoutFilter(t *testing.T) {
	t.Setenv("RESUME_COMPLETENESS_THRESHOLD", "60")
	facts := &stubResumeFacts{byUser: map[uint]models.ResumeLatestFact{
		2: {HasDocument: true}, // レビュー未実施
	}}
	res, err := resumeService(facts, threeStudents...).ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(res.Students) != 3 {
		t.Fatalf("全員返すこと: %d", len(res.Students))
	}
	for _, s := range res.Students {
		if s.ResumeStatus == nil {
			t.Fatalf("resume_status が無い: user=%d", s.UserID)
		}
		switch s.UserID {
		case 1:
			if !s.ResumeStatus.NeedsAttention || s.ResumeStatus.Reason != "未提出" {
				t.Errorf("user1: %+v", s.ResumeStatus)
			}
		case 2:
			if !s.ResumeStatus.NeedsAttention || s.ResumeStatus.Reason != "レビュー未実施" {
				t.Errorf("user2: %+v", s.ResumeStatus)
			}
		case 3:
			if !s.ResumeStatus.NeedsAttention || s.ResumeStatus.Reason != "未提出" {
				t.Errorf("user3: %+v", s.ResumeStatus)
			}
		}
	}
}

// 生徒数によらず1回しか呼ばない（N+1にしない）。
func TestListTendencies_ResumeFactsAreBatched(t *testing.T) {
	facts := &stubResumeFacts{}
	if _, err := resumeService(facts, threeStudents...).ListTendencies(25, 0, "", nil); err != nil {
		t.Fatalf("error = %v", err)
	}
	if facts.calls != 1 {
		t.Errorf("呼び出し回数 = %d, want 1", facts.calls)
	}
	if len(facts.gotIDs) != 3 {
		t.Errorf("全生徒分をまとめて渡すこと: %v", facts.gotIDs)
	}
}

// 未注入なら resume_status は付けず、フィルタも全員を通す。
func TestListTendencies_ResumeDisabledWithoutReader(t *testing.T) {
	res, err := resumeService(nil, threeStudents...).ListTendenciesWithFilters(25, 0, "", nil, false, true)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	// reader 無しでは NeedsAttention を判定できないので、フィルタは誰も残さない
	// （誤って全員を「要対応」扱いしない）。
	if len(res.Students) != 0 {
		t.Fatalf("reader 無しでフィルタ時は0件: %d", len(res.Students))
	}
	resAll, err := resumeService(nil, threeStudents...).ListTendencies(25, 0, "", nil)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	for _, s := range resAll.Students {
		if s.ResumeStatus != nil {
			t.Errorf("未注入なのに resume_status が付いている: %+v", s.ResumeStatus)
		}
	}
}

// 先頭ページに該当が無くても、後段の要対応生徒を落とさないこと（#1536 review）。
// ページ取得→絞り込みだと limit=25 のとき 26人目が要対応でも total=0 になる。
func TestListTendencies_ResumeFilterPagesAfterMatch(t *testing.T) {
	t.Setenv("RESUME_COMPLETENESS_THRESHOLD", "60")

	students := make([]entity.User, 0, 26)
	for i := 1; i <= 26; i++ {
		students = append(students, entity.User{
			ID:    uint(i),
			Name:  "生徒",
			Email: "s@example.com",
		})
	}
	// 1〜25 は提出済み・閾値以上。26 だけ未提出（map に載せない）。
	ok := 80
	facts := map[uint]models.ResumeLatestFact{}
	for i := 1; i <= 25; i++ {
		score := ok
		facts[uint(i)] = models.ResumeLatestFact{HasDocument: true, LatestScore: &score}
	}
	lister := &pagingLister{students: students}
	svc := NewStudentInsightService(
		lister,
		&stubScores{scores: map[uint]map[string]float64{}},
		&stubIndustries{},
		&stubProfiles{},
	)
	svc.SetResumeFactReader(&stubResumeFacts{byUser: facts})

	res, err := svc.ListTendenciesWithFilters(25, 0, "", nil, false, true)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if lister.gotLimit != filteredListScanLimit || lister.gotOffset != 0 {
		t.Fatalf("フィルタ時は先読みすること: limit=%d offset=%d", lister.gotLimit, lister.gotOffset)
	}
	if res.Total != 1 || len(res.Students) != 1 || res.Students[0].UserID != 26 {
		t.Fatalf("26人目の要対応を返すこと: total=%d students=%+v", res.Total, res.Students)
	}
}

// pagingLister は limit/offset を実際に切る。ページ後絞り込みバグの再現用。
type pagingLister struct {
	students            []entity.User
	gotLimit, gotOffset int
}

func (s *pagingLister) ListStudentsPaged(limit, offset int, _ string, _ *uint) ([]entity.User, int64, error) {
	s.gotLimit, s.gotOffset = limit, offset
	total := int64(len(s.students))
	if offset >= len(s.students) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(s.students) {
		end = len(s.students)
	}
	return s.students[offset:end], total, nil
}
