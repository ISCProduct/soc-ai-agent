package interview

import (
	"errors"
	"testing"
	"time"
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
