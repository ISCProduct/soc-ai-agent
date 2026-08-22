package interview

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/shared"

	"github.com/stretchr/testify/assert"
)

// sessionRepoStub は InterviewSessionRepository の最小実装。
// FinishSession/StartSession/Turn/StartTurn の状態機械テスト(#1019)専用。
type sessionRepoStub struct {
	sessions    map[uint]*models.InterviewSession
	updateCalls int
}

func newSessionRepoStub(sessions ...*models.InterviewSession) *sessionRepoStub {
	m := make(map[uint]*models.InterviewSession, len(sessions))
	for _, s := range sessions {
		m[s.ID] = s
	}
	return &sessionRepoStub{sessions: m}
}

func (r *sessionRepoStub) Create(session *models.InterviewSession) error {
	r.sessions[session.ID] = session
	return nil
}

func (r *sessionRepoStub) FindByID(id uint) (*models.InterviewSession, error) {
	if s, ok := r.sessions[id]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (r *sessionRepoStub) Update(session *models.InterviewSession) error {
	r.updateCalls++
	r.sessions[session.ID] = session
	return nil
}

func (r *sessionRepoStub) ListByUser(userID uint, limit int, offset int) ([]models.InterviewSession, error) {
	return nil, nil
}

func (r *sessionRepoStub) ListAll(limit int, offset int, schoolID *uint) ([]models.InterviewSession, error) {
	return nil, nil
}

func (r *sessionRepoStub) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	return nil, nil
}

func (r *sessionRepoStub) CountByUser(userID uint) (int64, error) { return 0, nil }
func (r *sessionRepoStub) CountAll(schoolID *uint) (int64, error) { return 0, nil }
func (r *sessionRepoStub) CountByUserAndDay(userID uint, day time.Time) (int64, error) {
	return 0, nil
}

func newTestInterviewService(repo *sessionRepoStub) *InterviewService {
	svc := NewInterviewService(repo, nil, nil, nil, nil, nil, nil)
	return svc
}

func TestFinishSession_FirstCall_EnqueuesReport(t *testing.T) {
	t.Parallel()

	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})
	svc := newTestInterviewService(repo)

	resp, err := svc.FinishSession(10, 1)
	assert.NoError(t, err)
	assert.Equal(t, "finished", resp.Status)
	assert.Equal(t, 1, repo.updateCalls, "初回終了ではUpdateが1回呼ばれる")
	assert.Equal(t, 1, len(svc.jobCh), "初回終了ではレポート生成が1回キューされる")
}

// TestFinishSession_SecondCall_NoReenqueue は #1019 の中心的な回帰テスト:
// 既に finished のセッションへ Finish を再実行しても、Update・レポート再キューが起きないことを確認する。
func TestFinishSession_SecondCall_NoReenqueue(t *testing.T) {
	t.Parallel()

	endedAt := time.Now().Add(-time.Minute)
	repo := newSessionRepoStub(&models.InterviewSession{
		ID: 1, UserID: 10, Status: "finished", EndedAt: &endedAt, EstimatedCostUSD: 1.23,
	})
	svc := newTestInterviewService(repo)

	resp, err := svc.FinishSession(10, 1)
	assert.NoError(t, err)
	assert.Equal(t, "finished", resp.Status)
	assert.Equal(t, 0, repo.updateCalls, "既にfinishedならUpdateを呼ばない(冪等)")
	assert.Equal(t, 0, len(svc.jobCh), "既にfinishedならレポートを再キューしない")
}

func TestStartSession_RejectsFinishedSession(t *testing.T) {
	t.Parallel()

	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "finished"})
	svc := newTestInterviewService(repo)

	_, err := svc.StartSession(10, 1)
	assert.ErrorIs(t, err, shared.ErrSessionFinished)
}

func TestTurn_RejectsFinishedSession(t *testing.T) {
	t.Parallel()

	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "finished"})
	svc := newTestInterviewService(repo)

	// openaiClient は nil のまま: finished ガードが先に return するため触れられないことも同時に検証する。
	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), nil,
		"企業名", "", "position", "info", "general", 0, 0, 60, 0, 0, 0, 0)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, shared.ErrSessionFinished)
}

func TestStartTurn_RejectsFinishedSession(t *testing.T) {
	t.Parallel()

	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "finished"})
	svc := newTestInterviewService(repo)

	result, err := svc.StartTurn(context.Background(), 10, 1,
		"企業名", "", "position", "info", "general", 0, 0, 0, 0, 0)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, shared.ErrSessionFinished)
}

func TestTurn_AllowsInProgressSession_UpToFinishedCheck(t *testing.T) {
	t.Parallel()

	// finished でなければガードを通過すること自体は確認できる
	// (この先の openaiClient 呼び出しは nil pointer で落ちるため、ガード通過だけを検証する)
	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})
	svc := newTestInterviewService(repo)

	defer func() {
		r := recover()
		assert.NotNil(t, r, "openaiClientがnilのためガード通過後にpanicする想定")
	}()
	_, _ = svc.Turn(context.Background(), 10, 1, []byte("audio"), nil,
		"企業名", "", "position", "info", "general", 0, 0, 60, 0, 0, 0, 0)
}
