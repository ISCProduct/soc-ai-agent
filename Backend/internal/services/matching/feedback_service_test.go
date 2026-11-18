package matching

import (
	"errors"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"
)

type memFeedbackStore struct {
	rows    map[[2]uint]models.UserMatchFeedback
	deleted [][2]uint
	readErr error
	match   *models.UserCompanyMatch
	findErr error
}

func (s *memFeedbackStore) FindMatch(uint) (*models.UserCompanyMatch, error) {
	return s.match, s.findErr
}

func (s *memFeedbackStore) Upsert(f *models.UserMatchFeedback) error {
	if s.rows == nil {
		s.rows = map[[2]uint]models.UserMatchFeedback{}
	}
	s.rows[[2]uint{f.UserID, f.MatchID}] = *f
	return nil
}
func (s *memFeedbackStore) Delete(u, m uint) error {
	s.deleted = append(s.deleted, [2]uint{u, m})
	delete(s.rows, [2]uint{u, m})
	return nil
}
func (s *memFeedbackStore) VerdictsByMatchIDs(u uint, ids []uint) (map[uint]string, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	out := map[uint]string{}
	for _, id := range ids {
		if r, ok := s.rows[[2]uint{u, id}]; ok {
			out[id] = r.Verdict
		}
	}
	return out, nil
}

func newFeedbackService(match *models.UserCompanyMatch, err error) (*MatchFeedbackService, *memFeedbackStore) {
	st := &memFeedbackStore{match: match, findErr: err}
	return NewMatchFeedbackService(st), st
}

func TestMatchFeedback_SavesVerdictWithSnapshot(t *testing.T) {
	svc, st := newFeedbackService(&models.UserCompanyMatch{ID: 5, UserID: 7, MatchScore: 71.5, FitMismatchAxes: "技術志向,安定志向", JobMatched: true}, nil)
	if err := svc.Submit(7, 5, "unfit"); err != nil {
		t.Fatal(err)
	}
	got := st.rows[[2]uint{7, 5}]
	if got.Verdict != "unfit" || got.MatchScore != 71.5 || got.FitMismatchAxes != "技術志向,安定志向" || !got.JobMatched {
		t.Fatalf("押した時点の写しを残すはず: %+v", got)
	}
	// 押し直すと上書き
	if err := svc.Submit(7, 5, "fit"); err != nil || st.rows[[2]uint{7, 5}].Verdict != "fit" {
		t.Fatalf("上書き: %v %+v", err, st.rows)
	}
}

func TestMatchFeedback_NoneDeletes(t *testing.T) {
	svc, st := newFeedbackService(&models.UserCompanyMatch{ID: 5, UserID: 7}, nil)
	_ = svc.Submit(7, 5, "fit")
	if err := svc.Submit(7, 5, MatchFeedbackNone); err != nil {
		t.Fatal(err)
	}
	if len(st.rows) != 0 || len(st.deleted) != 1 {
		t.Fatalf("取り消しで消えるはず: rows=%v deleted=%v", st.rows, st.deleted)
	}
}

func TestMatchFeedback_Rejects(t *testing.T) {
	tests := []struct {
		name    string
		match   *models.UserCompanyMatch
		findErr error
		verdict string
		want    error
	}{
		{"知らない判定", &models.UserCompanyMatch{UserID: 7}, nil, "great", ErrInvalidFeedbackVerdict},
		{"他人の推薦には付けられない", &models.UserCompanyMatch{UserID: 8}, nil, "fit", shared.ErrForbidden},
		{"推薦が無い", nil, repository.ErrNotFound, "fit", shared.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, st := newFeedbackService(tt.match, tt.findErr)
			if err := svc.Submit(7, 5, tt.verdict); !errors.Is(err, tt.want) {
				t.Fatalf("err=%v, want %v", err, tt.want)
			}
			if len(st.rows) != 0 {
				t.Fatalf("保存してはいけない: %v", st.rows)
			}
		})
	}
}

func TestMatchFeedback_VerdictsDoNotBlockOnReadError(t *testing.T) {
	svc, st := newFeedbackService(&models.UserCompanyMatch{ID: 5, UserID: 7}, nil)
	_ = svc.Submit(7, 5, "fit")
	if got := svc.Verdicts(7, []uint{5, 6}); got[5] != "fit" || len(got) != 1 {
		t.Fatalf("%v", got)
	}
	// 本人以外の判定は引けない
	if got := svc.Verdicts(8, []uint{5}); len(got) != 0 {
		t.Fatalf("他人の判定が見えた: %v", got)
	}
	st.readErr = errors.New("db")
	if got := svc.Verdicts(7, []uint{5}); got == nil || len(got) != 0 {
		t.Fatalf("読めなくても空を返す: %v", got)
	}
}
