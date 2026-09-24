package interview

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
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
}

func (r *reportRepoStub) FindBySessionID(sessionID uint) (*models.InterviewReport, error) {
	if r.last == nil {
		return nil, errors.New("not found")
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
