package application

// 応募・選考更新 → 集合知 RecordAction の配線テスト。
// 実行: cd Backend && go test ./internal/services/application/ -run Collective -v

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

type stubCollective struct {
	mu    sync.Mutex
	calls []collectiveCall
}

type collectiveCall struct {
	userID     uint
	sessionID  string
	companyID  uint
	actionType string
}

func (s *stubCollective) RecordAction(userID uint, sessionID string, companyID uint, actionType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, collectiveCall{userID, sessionID, companyID, actionType})
	return nil
}

func TestCollectiveActionForStatus(t *testing.T) {
	tests := []struct {
		status string
		want   string
		ok     bool
	}{
		{"applied", "applied", true},
		{"document_passed", "passed", true},
		{"offered", "passed", true},
		{"accepted", "passed", true},
		{"rejected", "rejected", true},
		{"document_screening", "", false},
		{"withdrawn", "", false},
		{"interview", "", false}, // legacy → interview_in_progress（通過ではない）
	}
	for _, tt := range tests {
		got, ok := collectiveActionForStatus(tt.status)
		if ok != tt.ok || got != tt.want {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", tt.status, got, ok, tt.want, tt.ok)
		}
	}
}

func TestApply_RecordsCollectiveApplied(t *testing.T) {
	db, mock := newSQLMockDB(t)
	repo := &stubApplyMatchRepo{
		findByIDResult: &models.UserCompanyMatch{ID: 5, UserID: 1, CompanyID: 10, SessionID: "sess-a"},
	}
	collective := &stubCollective{}
	s := &ApplicationService{appRepo: repositories.NewUserApplicationStatusRepository(db), matchRepo: repo, collective: collective}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `user_application_statuses` WHERE user_id = \\? AND company_id = \\? AND status NOT IN \\(\\?,\\?,\\?\\)").
		WithArgs(uint(1), uint(10), "accepted", "withdrawn", "rejected", sqlmock.AnyArg()).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectExec("INSERT INTO `user_application_statuses`").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec("UPDATE `user_company_matches` SET `is_applied`=\\?").
		WithArgs(true, sqlmock.AnyArg(), uint(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := s.Apply(1, 10, 5); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	collective.mu.Lock()
	defer collective.mu.Unlock()
	if len(collective.calls) != 1 {
		t.Fatalf("collective calls=%d want 1", len(collective.calls))
	}
	got := collective.calls[0]
	if got.actionType != "applied" || got.sessionID != "sess-a" || got.userID != 1 || got.companyID != 10 {
		t.Fatalf("%+v", got)
	}
}

func TestRecordCollectiveAction_NilSafe(t *testing.T) {
	s := &ApplicationService{}
	s.recordCollectiveAction(1, 2, "s", "applied") // panic しない
}

func TestRecordCollectiveAction_CallsRecorder(t *testing.T) {
	c := &stubCollective{}
	s := &ApplicationService{collective: c}
	s.recordCollectiveAction(9, 3, "sess-x", "passed")
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.calls) != 1 {
		t.Fatalf("calls=%d", len(c.calls))
	}
	got := c.calls[0]
	if got.userID != 9 || got.companyID != 3 || got.sessionID != "sess-x" || got.actionType != "passed" {
		t.Fatalf("%+v", got)
	}
}
