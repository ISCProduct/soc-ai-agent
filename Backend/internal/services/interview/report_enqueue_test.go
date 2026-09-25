package interview

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"Backend/internal/models"
)

// stubJobEnqueuer は shared.JobEnqueuer の最小スタブ。EnqueueInterviewReport 以外は使わない。
type stubJobEnqueuer struct {
	err           error
	reportCalls   int
	lastSessionID uint
}

func (s *stubJobEnqueuer) EnqueueEmailVerification(uint, string, string, string, string) error {
	return nil
}
func (s *stubJobEnqueuer) EnqueueEmailReVerification(uint, string, string, string, string) error {
	return nil
}
func (s *stubJobEnqueuer) EnqueueEmailRegistration(string, string) error          { return nil }
func (s *stubJobEnqueuer) EnqueueEmailPasswordReset(string, string, string) error { return nil }
func (s *stubJobEnqueuer) EnqueueInterviewReport(sessionID uint) error {
	s.reportCalls++
	s.lastSessionID = sessionID
	return s.err
}

func (s *stubJobEnqueuer) EnqueueDiagnosisQuality(uint, string) error { return nil }

func newEnqueueTestService() *InterviewService {
	return NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
}

// fillJobCh は jobCh をバッファ満杯にする。
func fillJobCh(t *testing.T, s *InterviewService) {
	t.Helper()
	for i := 0; i < jobChBufferSize; i++ {
		if !s.offerReportJob(uint(i + 1)) {
			t.Fatalf("バッファを埋める途中で投入に失敗した (i=%d)", i)
		}
	}
}

// TestEnqueueReportGeneration_DoesNotBlockWhenChannelFull は #1062 の受け入れ条件。
// バッファ満杯でも FinishSession 経路がハングしないことを検証する。
func TestEnqueueReportGeneration_DoesNotBlockWhenChannelFull(t *testing.T) {
	svc := newEnqueueTestService()
	fillJobCh(t, svc)

	done := make(chan struct{})
	go func() {
		svc.enqueueReportGeneration(9999)
		close(done)
	}()

	select {
	case <-done:
		// 期待どおり即座に戻る
	case <-time.After(2 * time.Second):
		t.Fatal("バッファ満杯で enqueueReportGeneration がブロックした（面接終了APIがハングする）")
	}
}

func TestOfferReportJob(t *testing.T) {
	t.Run("空きがあれば投入してtrueを返す", func(t *testing.T) {
		svc := newEnqueueTestService()
		if !svc.offerReportJob(42) {
			t.Fatal("空きがあるのに投入に失敗した")
		}
		select {
		case got := <-svc.jobCh:
			if got != 42 {
				t.Fatalf("sessionID=%d want 42", got)
			}
		default:
			t.Fatal("jobCh に投入されていない")
		}
	})

	t.Run("満杯なら捨ててfalseを返す", func(t *testing.T) {
		svc := newEnqueueTestService()
		fillJobCh(t, svc)
		if svc.offerReportJob(9999) {
			t.Fatal("満杯なのに投入に成功したと報告された")
		}
		if len(svc.jobCh) != jobChBufferSize {
			t.Fatalf("バッファ長=%d want %d（溢れ分が入り込んでいる）", len(svc.jobCh), jobChBufferSize)
		}
	})
}

func TestEnqueueReportGeneration_PrefersJobEnqueuer(t *testing.T) {
	svc := newEnqueueTestService()
	jobs := &stubJobEnqueuer{}
	svc.SetJobEnqueuer(jobs)

	svc.enqueueReportGeneration(7)

	if jobs.reportCalls != 1 || jobs.lastSessionID != 7 {
		t.Fatalf("EnqueueInterviewReport calls=%d sessionID=%d want 1, 7", jobs.reportCalls, jobs.lastSessionID)
	}
	if len(svc.jobCh) != 0 {
		t.Fatalf("Redis 経路が成功したのに channel へも投入されている (len=%d)", len(svc.jobCh))
	}
}

func TestEnqueueReportGeneration_FallsBackToChannelOnEnqueueError(t *testing.T) {
	svc := newEnqueueTestService()
	svc.SetJobEnqueuer(&stubJobEnqueuer{err: errors.New("redis down")})

	svc.enqueueReportGeneration(7)

	select {
	case got := <-svc.jobCh:
		if got != 7 {
			t.Fatalf("sessionID=%d want 7", got)
		}
	default:
		t.Fatal("Redis 失敗時に channel へフォールバックしていない")
	}
}

// TestEnqueueReportGeneration_DoesNotBlockWhenRedisFailsAndChannelFull は
// Redis 障害とバッファ満杯が重なった最悪ケースでもブロックしないことを検証する。
func TestEnqueueReportGeneration_DoesNotBlockWhenRedisFailsAndChannelFull(t *testing.T) {
	svc := newEnqueueTestService()
	svc.SetJobEnqueuer(&stubJobEnqueuer{err: errors.New("redis down")})
	fillJobCh(t, svc)

	done := make(chan struct{})
	go func() {
		svc.enqueueReportGeneration(9999)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Redis障害 + バッファ満杯で enqueueReportGeneration がブロックした")
	}
}

// TestRegenerateReport_QueueFullIsNotSuccess は #1476 のレビュー指摘の回帰テスト。
//
// フォールバックキューが満杯だとジョブは捨てられるが、これを 202 queued=true で返すと
// 唯一の回復操作である再試行APIが成功に見え、フロントは存在しないジョブを3分ポーリングして
// また同じタイムアウト画面に戻る。キュー単体の false ではなく、再生成API経路
// （RegenerateReport）を通して「投入失敗はエラーになる」ことを固定する。
func TestRegenerateReport_QueueFullIsNotSuccess(t *testing.T) {
	// fillJobCh が 1..jobChBufferSize を使うため、重複排除に引っ掛からないIDを選ぶ。
	const ownerID, sessionID = uint(3), uint(jobChBufferSize + 7)

	tests := []struct {
		name string
		jobs *stubJobEnqueuer
	}{
		{name: "Redis未設定でキュー満杯", jobs: nil},
		{name: "Redis障害中でキュー満杯", jobs: &stubJobEnqueuer{err: errors.New("redis down")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewInterviewService(
				newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: ownerID, Status: "finished"}),
				&utterRepoStub{},
				&reportRepoStub{},
				nonAdminUserRepoStub{},
				nil, nil, nil,
			)
			if tt.jobs != nil {
				svc.SetJobEnqueuer(tt.jobs)
			}
			fillJobCh(t, svc)

			queued, err := svc.RegenerateReport(ownerID, sessionID)

			if !errors.Is(err, ErrReportQueueNotAvailable) {
				t.Fatalf("err=%v want errors.Is(err, ErrReportQueueNotAvailable)（投入失敗が成功として返っている）", err)
			}
			if queued {
				t.Fatal("ジョブを捨てたのに queued=true を返した（フロントが存在しないジョブを待ち続ける）")
			}
		})
	}
}

// blockingUtterRepo は generateReport を任意の時点まで止めておくための発話リポジトリ。
// LLM 呼び出しまで進む前に失敗させるため、解放後は取得エラーを返す。
type blockingUtterRepo struct {
	started chan uint
	release chan struct{}
	calls   atomic.Int32
}

func (r *blockingUtterRepo) Create(*models.InterviewUtterance) error { return nil }

func (r *blockingUtterRepo) FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error) {
	r.calls.Add(1)
	r.started <- sessionID
	<-r.release
	return nil, errors.New("レポート生成失敗（テスト用）")
}

// TestFallbackWorker_DoesNotRunSameSessionTwice は #1476 の回帰テスト。
//
// Redis 未設定・障害時の channel フォールバックには asynq の重複排除が無い。
// 生成が3分のUIタイムアウトを超えて走っている間に再試行ボタンを押されると、
// レポートはまだ未保存なので RegenerateReport が通り、同じセッションが2回実行されてしまう。
// LLM 費用の二重化・レポート上書き・スコアの二重反映が起きるため、
// worker を実際に起動した本番経路（runWorker → generateReport）で重複しないことを固定する。
//
// 同時に、失敗したジョブの後は再生成できること（回復手段が塞がらないこと）も確認する。
func TestFallbackWorker_DoesNotRunSameSessionTwice(t *testing.T) {
	const ownerID, sessionID = uint(3), uint(7)

	utterRepo := &blockingUtterRepo{started: make(chan uint), release: make(chan struct{})}
	t.Cleanup(func() { close(utterRepo.release) })

	svc := NewInterviewService(
		newSessionRepoStub(&models.InterviewSession{ID: sessionID, UserID: ownerID, Status: "finished"}),
		utterRepo,
		&reportRepoStub{},
		nonAdminUserRepoStub{},
		nil, nil, nil,
	)
	svc.StartWorker()

	if _, err := svc.RegenerateReport(ownerID, sessionID); err != nil {
		t.Fatalf("再生成の投入に失敗: %v", err)
	}
	select {
	case <-utterRepo.started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker がレポート生成を開始していない")
	}

	// 生成中にもう一度再試行ボタンを押す。レポートはまだ未保存なので RegenerateReport は通る。
	if _, err := svc.RegenerateReport(ownerID, sessionID); err != nil {
		t.Fatalf("再生成の投入に失敗: %v", err)
	}
	if n := len(svc.jobCh); n != 0 {
		t.Fatalf("処理中の同一セッションが %d 件追加投入された（二重実行になる）", n)
	}

	// 1件目を失敗で終わらせる。ここで2件目が控えていれば worker がそのまま続けて実行する。
	utterRepo.release <- struct{}{}
	select {
	case <-utterRepo.started:
		t.Fatal("処理中に積まれた重複ジョブが続けて実行された（LLM費用とスコア反映が二重になる）")
	case <-time.After(500 * time.Millisecond):
	}

	// 失敗したジョブの後は再生成できなければ回復手段が無くなる。
	if _, err := svc.RegenerateReport(ownerID, sessionID); err != nil {
		t.Fatalf("再生成の投入に失敗: %v", err)
	}
	select {
	case <-utterRepo.started:
		utterRepo.release <- struct{}{}
	case <-time.After(2 * time.Second):
		t.Fatal("失敗したジョブの再生成が実行されない（回復手段が塞がっている）")
	}
	if got := utterRepo.calls.Load(); got != 2 {
		t.Fatalf("generateReport 実行回数=%d want 2", got)
	}
}
