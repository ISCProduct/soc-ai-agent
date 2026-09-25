package interview

import (
	"context"
	"errors"
	"testing"

	"Backend/domain/entity"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"

	"gorm.io/gorm"
)

// utterRepoStub は InterviewUtteranceRepository の最小実装。
type utterRepoStub struct {
	utterances []models.InterviewUtterance
	findErr    error
	createErr  error
}

func (r *utterRepoStub) Create(u *models.InterviewUtterance) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.utterances = append(r.utterances, *u)
	return nil
}

func (r *utterRepoStub) FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.utterances, nil
}

// reportRepoStub は InterviewReportRepository の最小実装。Upsert 回数を数える。
type reportRepoStub struct {
	upsertCalls int
	last        *models.InterviewReport
	findErr     error
}

func (r *reportRepoStub) FindBySessionID(sessionID uint) (*models.InterviewReport, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.last == nil {
		// 実リポジトリ(gorm First)と同じエラーを返す。ここを generic なエラーにすると
		// 「未生成」と「DB障害」の区別が付かず、RegenerateReport の分岐がテストを素通りする。
		return nil, gorm.ErrRecordNotFound
	}
	return r.last, nil
}

func (r *reportRepoStub) Upsert(report *models.InterviewReport) error {
	r.upsertCalls++
	r.last = report
	return nil
}

// TestGenerateReport_NoUtterances は #1476 の中心的な回帰テスト。
//
// 発話が0件のセッションは「面接に中身が無かった」とは限らず、
// 発話保存(POST /utterances)が落ちて欠落しただけの可能性がある。
// ここで空レポートを保存して正常終了すると、ユーザーには中身のないレポートが
// 成功として届き、失敗の痕跡がどこにも残らない。
// 空レポートを保存せずエラーを返し、ジョブ側の再試行に委ねる。
func TestGenerateReport_NoUtterances(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		utterRepo     *utterRepoStub
		wantErr       error
		wantErrAny    bool
		wantUpsert    int
		wantUpsertMsg string
	}{
		{
			name:          "発話0件なら空レポートを保存せず再試行可能なエラーを返す",
			utterRepo:     &utterRepoStub{},
			wantErr:       ErrNoUtterances,
			wantUpsert:    0,
			wantUpsertMsg: "発話0件で空レポートが保存されている",
		},
		{
			name:          "発話の取得自体が失敗したらエラーを返す",
			utterRepo:     &utterRepoStub{findErr: errors.New("db down")},
			wantErrAny:    true,
			wantUpsert:    0,
			wantUpsertMsg: "取得失敗なのにレポートが保存されている",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: 7, UserID: 3, Status: "finished"})
			reportRepo := &reportRepoStub{}
			svc := NewInterviewService(sessionRepo, tt.utterRepo, reportRepo, nil, nil, nil, nil)

			err := svc.GenerateReportForSession(context.Background(), 7)

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v want errors.Is(err, %v)", err, tt.wantErr)
			}
			if tt.wantErrAny && err == nil {
				t.Fatal("エラーを期待したが nil が返った")
			}
			if reportRepo.upsertCalls != tt.wantUpsert {
				t.Fatalf("Upsert 呼び出し=%d want %d: %s", reportRepo.upsertCalls, tt.wantUpsert, tt.wantUpsertMsg)
			}
		})
	}
}

// nonAdminUserRepoStub は isAllowed のため「管理者ではない一般ユーザー」を返すだけのスタブ。
// 未実装メソッドは埋め込んだインターフェース経由（呼ばれたら nil panic で気付ける）。
type nonAdminUserRepoStub struct{ repository.UserRepository }

func (nonAdminUserRepoStub) GetUserByID(id uint) (*entity.User, error) {
	return &entity.User{ID: id, IsAdmin: false}, nil
}

// TestRegenerateReport は #1476 のレビュー指摘「未生成レポートを実際に再生成できる経路」の回帰テスト。
//
// レポート生成ジョブは失われうる（asynq の再試行を使い切る、フォールバック worker で
// 発話0件により1回で落ちる、jobCh が溢れる）。FinishSession は終了済みセッションを
// 再キューしない(#1019)ため、これが唯一の回復経路。
// 逆に生成済みのレポートまで再投入すると LLM 費用が二重に掛かり、結果が上書きされる。
func TestRegenerateReport(t *testing.T) {
	t.Parallel()

	const ownerID, otherID, sessionID = uint(3), uint(99), uint(7)

	tests := []struct {
		name       string
		callerID   uint
		status     string
		existing   *models.InterviewReport
		findErr    error
		wantQueued bool
		wantErr    error
		wantErrAny bool
		wantJobs   int
		msg        string
	}{
		{
			name:       "終了済みでレポート未生成なら再投入する",
			callerID:   ownerID,
			status:     "finished",
			wantQueued: true,
			wantJobs:   1,
			msg:        "未生成なのに再投入されず、ユーザーに回復手段が無いまま",
		},
		{
			name:       "レポート生成済みなら再投入しない",
			callerID:   ownerID,
			status:     "finished",
			existing:   &models.InterviewReport{SessionID: sessionID},
			wantQueued: false,
			wantJobs:   0,
			msg:        "生成済みなのに再投入され、LLM費用が二重に掛かる",
		},
		{
			name:     "未終了セッションは再投入しない",
			callerID: ownerID,
			status:   "in_progress",
			wantErr:  ErrSessionNotFinished,
			wantJobs: 0,
			msg:      "面接中にレポート生成が走っている",
		},
		{
			name:     "他人のセッションは403",
			callerID: otherID,
			status:   "finished",
			wantErr:  shared.ErrForbidden,
			wantJobs: 0,
			msg:      "他人のセッションのレポート生成をキックできている",
		},
		{
			name:       "レポート取得がDB障害なら再投入せずエラーを返す",
			callerID:   ownerID,
			status:     "finished",
			findErr:    errors.New("db down"),
			wantErrAny: true,
			wantJobs:   0,
			msg:        "生成済みか判定できないのに再投入している",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sessionRepo := newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: ownerID, Status: tt.status})
			reportRepo := &reportRepoStub{last: tt.existing, findErr: tt.findErr}
			userRepo := nonAdminUserRepoStub{}
			svc := NewInterviewService(sessionRepo, &utterRepoStub{}, reportRepo, userRepo, nil, nil, nil)

			queued, err := svc.RegenerateReport(tt.callerID, sessionID)

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v want errors.Is(err, %v)", err, tt.wantErr)
			}
			if tt.wantErrAny && err == nil {
				t.Fatal("エラーを期待したが nil が返った")
			}
			if tt.wantErr == nil && !tt.wantErrAny && err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if queued != tt.wantQueued {
				t.Fatalf("queued=%v want %v", queued, tt.wantQueued)
			}
			if len(svc.jobCh) != tt.wantJobs {
				t.Fatalf("投入されたジョブ数=%d want %d: %s", len(svc.jobCh), tt.wantJobs, tt.msg)
			}
		})
	}
}
