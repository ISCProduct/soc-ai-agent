package interview

import (
	"context"
	"testing"
	"time"
)

func TestStopWorker_DrainsAndCanBeCalledTwice(t *testing.T) {
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	ran := make(chan uint, 1)
	svc.reportJob = func(ctx context.Context, sessionID uint) error {
		ran <- sessionID
		return nil
	}
	svc.StartWorker()
	if !svc.offerReportJob(4) {
		t.Fatal("ジョブを投入できない")
	}
	svc.StopWorker(context.Background())
	select {
	case id := <-ran:
		if id != 4 {
			t.Fatalf("実行した session=%d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("キューに残ったジョブが実行されない")
	}
	svc.StopWorker(context.Background())
	if svc.offerReportJob(5) {
		t.Fatal("停止後の投入は拒否する")
	}
}

func TestStopWorker_BlockingJobStopsAtDeadline(t *testing.T) {
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	svc.reportJob = func(ctx context.Context, sessionID uint) error {
		close(started)
		<-release
		return nil
	}
	svc.StartWorker()
	if !svc.offerReportJob(7) {
		t.Fatal("ジョブを投入できない")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("ジョブが始まらない")
	}
	// 実装が期限を無視してジョブ完了を待つ場合に、テスト自体が止まらないようにする。
	go func() {
		time.Sleep(1500 * time.Millisecond)
		close(release)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	svc.StopWorker(ctx)
	if time.Since(startedAt) > time.Second {
		t.Fatal("期限を過ぎても StopWorker が戻らない")
	}
}
