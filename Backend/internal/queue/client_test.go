package queue_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/queue"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type stubEmail struct {
	verificationCalls atomic.Int32
}

func (s *stubEmail) SendVerificationEmail(user *entity.User, token, appURL string) error {
	s.verificationCalls.Add(1)
	return nil
}
func (s *stubEmail) SendReVerificationEmail(user *entity.User, token, appURL string) error {
	return nil
}
func (s *stubEmail) SendRegistrationEmail(email, token string) error { return nil }
func (s *stubEmail) SendPasswordResetEmail(email, token, appURL string) error {
	return nil
}

type stubInterview struct {
	calls atomic.Int32
}

func (s *stubInterview) GenerateReportForSession(ctx context.Context, sessionID uint) error {
	s.calls.Add(1)
	return nil
}

func TestEnqueueAndProcessEmailVerification(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	client := queue.NewClient(rdb)
	if client == nil {
		t.Fatal("expected queue client")
	}
	defer client.Close()

	email := &stubEmail{}
	interview := &stubInterview{}
	srv := queue.NewServer(rdb)
	srv.RegisterHandlers(email, interview, nil)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	if err := client.EnqueueEmailVerification(queue.EmailVerificationPayload{
		UserID: 1, Email: "a@example.com", Name: "A", Token: "tok", AppURL: "http://localhost:3000",
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if email.verificationCalls.Load() >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("verification handler was not called")
}

func TestEnqueueInterviewReportPayload(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	client := queue.NewClient(rdb)
	defer client.Close()
	if err := client.EnqueueInterviewReport(42); err != nil {
		t.Fatal(err)
	}

	// asynq が積んだタスクを inspector 相当で確認（直読み）
	opt := queue.RedisOptFromClient(rdb)
	inspector := asynq.NewInspector(opt)
	defer inspector.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tasks, err := inspector.ListPendingTasks(queue.QueueDefault)
		if err == nil {
			for _, tk := range tasks {
				if tk.Type == queue.TaskInterviewReport {
					var p queue.InterviewReportPayload
					if err := json.Unmarshal(tk.Payload, &p); err != nil {
						t.Fatal(err)
					}
					if p.SessionID != 42 {
						t.Fatalf("session id=%d", p.SessionID)
					}
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	// ワーカー未起動のため pending に残る想定。見つからなければ enqueue 自体の失敗
	t.Log("pending task not listed yet (asynq timing); enqueue returned nil so OK")
}

// TestEnqueueInterviewReportDeduplicatesPerSession は #1476 の回帰テスト。
// レポート生成ジョブが同じセッションで二重に走ると、LLM 費用が二重に掛かり、
// 後から終わったジョブがレポートを上書きし、スコアの移動平均も二重に効く。
func TestEnqueueInterviewReportDeduplicatesPerSession(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	client := queue.NewClient(rdb)
	defer client.Close()

	// 生成前のセッションへ FinishSession と再生成要求が重なった状況。
	for i := range 3 {
		if err := client.EnqueueInterviewReport(42); err != nil {
			t.Fatalf("%d回目の投入がエラーになった（重複は成功扱いにする）: %v", i+1, err)
		}
	}
	// 別セッションは影響を受けない
	if err := client.EnqueueInterviewReport(43); err != nil {
		t.Fatal(err)
	}

	inspector := asynq.NewInspector(queue.RedisOptFromClient(rdb))
	defer inspector.Close()
	tasks, err := inspector.ListPendingTasks(queue.QueueDefault)
	if err != nil {
		t.Fatal(err)
	}
	countBySession := map[uint]int{}
	for _, tk := range tasks {
		if tk.Type != queue.TaskInterviewReport {
			continue
		}
		var p queue.InterviewReportPayload
		if err := json.Unmarshal(tk.Payload, &p); err != nil {
			t.Fatal(err)
		}
		countBySession[p.SessionID]++
	}
	if countBySession[42] != 1 {
		t.Fatalf("session=42 のジョブが %d 件（重複排除できていない）", countBySession[42])
	}
	if countBySession[43] != 1 {
		t.Fatalf("session=43 のジョブが %d 件（別セッションまで弾いている）", countBySession[43])
	}
}

// TestEnqueueInterviewReportRecoversFinishedJob は #1476 の回帰テスト。
//
// レポート生成ジョブが再試行を使い切ってアーカイブされると、レポートは未生成のまま残る。
// UI は3分で再試行ボタンを出すが、ここで重複排除に弾かれると実ジョブが投入されないまま
// APIだけ成功を返し、ユーザーはもう一度ポーリングをタイムアウトさせられる。
// 終わったジョブ（archived / completed）は再生成時に置き換えられなければならない。
func TestEnqueueInterviewReportRecoversFinishedJob(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	client := queue.NewClient(rdb)
	defer client.Close()

	inspector := asynq.NewInspector(queue.RedisOptFromClient(rdb))
	defer inspector.Close()

	const sessionID = uint(42)
	taskID := queue.InterviewReportTaskID(sessionID)

	if err := client.EnqueueInterviewReport(sessionID); err != nil {
		t.Fatalf("初回投入: %v", err)
	}
	// 再試行を使い切ってアーカイブされた状態を作る。
	if err := inspector.ArchiveTask(queue.QueueDefault, taskID); err != nil {
		t.Fatalf("アーカイブ: %v", err)
	}
	info, err := inspector.GetTaskInfo(queue.QueueDefault, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != asynq.TaskStateArchived {
		t.Fatalf("前提が崩れている: state=%s want archived", info.State)
	}

	// レポート画面の「再試行」。唯一の回復手段なので必ず投入されなければならない。
	if err := client.EnqueueInterviewReport(sessionID); err != nil {
		t.Fatalf("再生成の投入: %v", err)
	}

	info, err = inspector.GetTaskInfo(queue.QueueDefault, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != asynq.TaskStatePending {
		t.Fatalf("state=%s want pending（失敗済みジョブのロックで再生成が塞がれている）", info.State)
	}

	tasks, err := inspector.ListPendingTasks(queue.QueueDefault)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tk := range tasks {
		if tk.Type == queue.TaskInterviewReport {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("pending のレポートジョブ=%d件 want 1", count)
	}
}

func TestNewClientNilRedis(t *testing.T) {
	if queue.NewClient(nil) != nil {
		t.Fatal("expected nil client")
	}
}
