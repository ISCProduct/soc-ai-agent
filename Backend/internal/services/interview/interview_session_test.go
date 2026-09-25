package interview

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/shared"

	"github.com/stretchr/testify/assert"
)

// sessionRepoStub は InterviewSessionRepository の最小実装。
// FinishSession/StartSession/Turn/StartTurn の状態機械テスト(#1019)専用。
type sessionRepoStub struct {
	sessions      map[uint]*models.InterviewSession
	updateCalls   int
	lastCompanyID *uint
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

func (r *sessionRepoStub) ListAll(limit int, offset int, schoolID *uint, companyID *uint) ([]models.InterviewSession, error) {
	r.lastCompanyID = companyID
	return nil, nil
}

func (r *sessionRepoStub) ListFinishedByUser(userID uint, limit int) ([]models.InterviewSession, error) {
	return nil, nil
}

func (r *sessionRepoStub) CountByUser(userID uint) (int64, error) { return 0, nil }
func (r *sessionRepoStub) CountAll(schoolID *uint, companyID *uint) (int64, error) {
	r.lastCompanyID = companyID
	return 0, nil
}
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

	// finished でなければガードを通過することを検証する。
	//
	// 以前は openaiClient が nil のときガード通過後に panic していたため、panic の有無で
	// 判定していた。#1293 で AI 未設定は panic ではなく ErrAIUnavailable による縮退に
	// なったため、ガードを通過して面接の縮退応答が返ることで判定する。
	repo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})
	svc := newTestInterviewService(repo)

	result, err := svc.Turn(context.Background(), 10, 1, []byte("audio"), nil,
		"企業名", "", "position", "info", "general", 0, 0, 60, 0, 0, 0, 0)

	// AI が使えないときは面接を落とさず、聞き取れなかった旨の応答で続行する（既存の縮退挙動）
	assert.NoError(t, err)
	assert.NotNil(t, result, "finishedガードを通過していない")
	assert.Contains(t, result.UserText, "聞き取れませんでした")
}

func TestListSessionsForOwner_ForbiddenWhenCheckerMissing(t *testing.T) {
	svc := newTestInterviewService(newSessionRepoStub())
	_, _, err := svc.ListSessionsForOwner(1, 10, 20, 0)
	assert.ErrorIs(t, err, shared.ErrForbidden)
}

func TestListSessionsForOwner_ForbiddenWhenNotOwner(t *testing.T) {
	svc := newTestInterviewService(newSessionRepoStub())
	svc.SetCompanyOwnerChecker(func(uint, uint) (bool, error) { return false, nil })
	_, _, err := svc.ListSessionsForOwner(1, 99, 20, 0)
	assert.ErrorIs(t, err, shared.ErrForbidden)
}

func TestListSessionsForOwner_FiltersCompanyID(t *testing.T) {
	repo := newSessionRepoStub()
	svc := newTestInterviewService(repo)
	svc.SetCompanyOwnerChecker(func(userID, companyID uint) (bool, error) {
		return userID == 1 && companyID == 10, nil
	})
	_, _, err := svc.ListSessionsForOwner(1, 10, 20, 0)
	assert.NoError(t, err)
	if repo.lastCompanyID == nil || *repo.lastCompanyID != 10 {
		t.Fatalf("company_id filter = %v want 10", repo.lastCompanyID)
	}
}

// 月次上限のガードが未設定なら面接は開始できる。
// ガードは任意の仕組みで、注入しない環境まで止めてはいけない。
func TestCreateSession_NoBudgetGuard(t *testing.T) {
	svc := &InterviewService{}
	if svc.budgetGuard != nil {
		t.Error("既定でガードが入っている")
	}
}

type stubBudgetGuard struct {
	err    error
	called int
}

func (g *stubBudgetGuard) AllowStart() error {
	g.called++
	return g.err
}

// ガードが拒否したら面接を作らない。
// ユーザー取得より前に判定して、無駄なクエリを避ける。
func TestCreateSession_BudgetGuardBlocks(t *testing.T) {
	guard := &stubBudgetGuard{err: errors.New("over limit")}
	svc := &InterviewService{}
	svc.SetBudgetGuard(guard)

	// userRepo は未設定。ガードで弾かれれば、そこへ到達せず panic しない
	_, err := svc.CreateSession(1, "ja", "female")
	if !errors.Is(err, ErrInterviewBudgetExceeded) {
		t.Errorf("err = %v, want ErrInterviewBudgetExceeded", err)
	}
	if guard.called != 1 {
		t.Errorf("ガードの呼び出し = %d, want 1", guard.called)
	}
}

// TestSaveUtterance_ClientUtteranceID は #1476 のレビュー指摘「再試行する発話保存を冪等に」の回帰テスト。
//
// POST /utterances は追記APIなので、クライアントが再試行すると
// 「サーバーはDBへ書けたが応答だけ失われた」ケースで同じ発話がもう一件入り、
// 書き起こし・スコア・LoRA学習データへ重複して伝播する。
// クライアントが発話ごとに発行するIDをそのまま行へ載せ、
// (session_id, client_utterance_id) の一意制約でサーバー側が再送を弾けるようにする。
func TestSaveUtterance_ClientUtteranceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		clientID  string
		wantStore *string
		wantErr   bool
		msg       string
	}{
		{
			name:      "IDを付けて送れば行に載る",
			clientID:  "9f1c2b3a-0000-4000-8000-000000000001",
			wantStore: strPtr("9f1c2b3a-0000-4000-8000-000000000001"),
			msg:       "IDが保存されないと一意制約が効かず再試行で二重保存される",
		},
		{
			name:      "前後の空白は落として保存する",
			clientID:  "  abc123  ",
			wantStore: strPtr("abc123"),
			msg:       "空白の有無で同じ発話が別IDになり二重保存される",
		},
		{
			name:      "ID未指定はNULLのまま（ID非対応の経路は従来どおり追記）",
			clientID:  "",
			wantStore: nil,
			msg:       "空文字を保存すると2件目以降が一意制約に当たって保存できなくなる",
		},
		{
			name:     "64文字を超えるIDは受け付けない",
			clientID: strings.Repeat("a", 65),
			wantErr:  true,
			msg:      "DBのVARCHAR(64)で黙って切り詰められ、別の発話と衝突しうる",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: 1, UserID: 10, Status: "in_progress"})
			utterRepo := &saveUtterRepoStub{}
			svc := NewInterviewService(sessionRepo, utterRepo, nil, nil, nil, nil, nil)

			err := svc.SaveUtterance(10, 1, "user", "自己紹介をします", tt.clientID)

			if tt.wantErr {
				assert.Error(t, err, tt.msg)
				assert.Empty(t, utterRepo.created, "エラーなのに保存されている")
				return
			}
			assert.NoError(t, err)
			if assert.Len(t, utterRepo.created, 1) {
				assert.Equal(t, tt.wantStore, utterRepo.created[0].ClientUtteranceID, tt.msg)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// saveUtterRepoStub は保存された行をそのまま記録するだけのスタブ。
type saveUtterRepoStub struct {
	created []models.InterviewUtterance
}

func (r *saveUtterRepoStub) Create(u *models.InterviewUtterance) error {
	r.created = append(r.created, *u)
	return nil
}

func (r *saveUtterRepoStub) FindBySessionID(uint) ([]models.InterviewUtterance, error) {
	return nil, nil
}
