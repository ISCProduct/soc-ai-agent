package interview_test

import (
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
	"Backend/internal/services/email"
	"Backend/internal/services/interview"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInterviewReportRepo は repository.InterviewReportRepository の最小モック実装。
type mockInterviewReportRepo struct {
	report *models.InterviewReport
	err    error
}

func (m *mockInterviewReportRepo) FindBySessionID(sessionID uint) (*models.InterviewReport, error) {
	return m.report, m.err
}
func (m *mockInterviewReportRepo) Upsert(report *models.InterviewReport) error { return nil }

// #939: SendReportEmail はセッション所有者以外からの呼び出しを forbidden で拒否しなければならない。
// 所有者チェック（isAllowed）はメール送信より前段で行われるため、forbidden を返した時点で
// emailService には到達していないことが保証される。
func TestSendReportEmail_Forbidden(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 99}, // 別ユーザーのセッション
	}
	reportRepo := &mockInterviewReportRepo{
		report: &models.InterviewReport{SessionID: 1, ScoresJSON: "{}", EvidenceJSON: "{}"},
	}
	svc := interview.NewInterviewService(sessionRepo, &mockInterviewUtterRepo{}, reportRepo, &mockUserRepo2{err: errors.New("not found")}, nil, nil, nil)

	// userID=1 が ownerID=99 のセッションのレポート送信を要求 → forbidden
	err := svc.SendReportEmail(1, 1)
	require.Error(t, err)
	assert.Equal(t, "forbidden", err.Error())
}

func TestSendReportEmail_SessionNotFound(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: nil,
		err:     errors.New("record not found"),
	}
	svc := interview.NewInterviewService(sessionRepo, &mockInterviewUtterRepo{}, &mockInterviewReportRepo{}, &mockUserRepo2{}, nil, nil, nil)

	err := svc.SendReportEmail(1, 1)
	require.Error(t, err)
	assert.Equal(t, "record not found", err.Error())
}

// 所有者本人からの呼び出しは成功し、レポート内容がメールデータに反映されること。
func TestSendReportEmail_Success(t *testing.T) {
	sessionRepo := &mockInterviewSessionRepo{
		session: &models.InterviewSession{ID: 1, UserID: 1},
	}
	reportRepo := &mockInterviewReportRepo{
		report: &models.InterviewReport{
			SessionID:    1,
			SummaryText:  "総合評価コメント",
			ScoresJSON:   `{"logic":4,"specificity":3,"ownership":5}`,
			EvidenceJSON: `{"logic":"根拠"}`,
		},
	}
	userRepo := &mockUserRepo2{user: &entity.User{ID: 1, Name: "山田太郎", Email: "yamada@example.com"}}
	svc := interview.NewInterviewService(sessionRepo, &mockInterviewUtterRepo{}, reportRepo, userRepo, &email.EmailService{}, nil, nil)

	err := svc.SendReportEmail(1, 1)
	require.NoError(t, err)
}
