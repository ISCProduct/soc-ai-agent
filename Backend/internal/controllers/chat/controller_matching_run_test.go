package chat

// バックグラウンドマッチングの打ち切りと多重実行防止のテスト（Issue #1167）
// 実行: cd Backend && go test ./internal/controllers/... -run TestRunBackgroundMatching -v

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"Backend/domain/entity"
	"Backend/internal/services/matching"
)

// stubMatchingService は CalculateMatching の呼び出しを記録する。
// block が非nilなら、その channel が閉じるまで計算中として振る舞う。
type stubMatchingService struct {
	calls   atomic.Int32
	entered chan struct{}
	block   chan struct{}
	mu      sync.Mutex
	lastCtx context.Context
}

func (s *stubMatchingService) CalculateMatching(ctx context.Context, userID uint, sessionID string) error {
	s.calls.Add(1)
	s.mu.Lock()
	s.lastCtx = ctx
	s.mu.Unlock()
	if s.entered != nil {
		select {
		case s.entered <- struct{}{}:
		default:
		}
	}
	if s.block != nil {
		<-s.block
	}
	return nil
}

func (s *stubMatchingService) GetTopMatches(context.Context, uint, string, int) ([]*entity.UserCompanyMatch, error) {
	return nil, nil
}
func (s *stubMatchingService) ToggleFavorite(uint, uint) error { return nil }
func (s *stubMatchingService) GetDiagnostics(uint, string) (*matching.MatchingDiagnostics, error) {
	return nil, nil
}

// TestRunBackgroundMatching_SkipsWhileRunning は実行中の同一セッションが
// 多重に計算されないことを検証する（#1167 の核心）。
func TestRunBackgroundMatching_SkipsWhileRunning(t *testing.T) {
	stub := &stubMatchingService{entered: make(chan struct{}, 1), block: make(chan struct{})}
	c := &ChatController{matchingService: stub}

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.runBackgroundMatching(1, "session-1")
	}()

	// 1本目が計算に入るまで待つ
	select {
	case <-stub.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("1本目が CalculateMatching に入らなかった")
	}

	// 実行中の2本目は即座に戻るべき。
	// 多重起動すると block で止まるため、直接呼ばずに時間で判定する（CI を10分ハングさせない）。
	second := make(chan struct{})
	go func() {
		defer close(second)
		c.runBackgroundMatching(1, "session-1")
	}()
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("実行中の2本目が戻らない（多重起動して計算に入っている）")
	}
	if got := stub.calls.Load(); got != 1 {
		t.Errorf("実行中に多重起動した: CalculateMatching 呼び出し回数=%d, want 1", got)
	}

	close(stub.block)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("1本目が終了しなかった")
	}

	// 完了後は実行中フラグが解放され、次の計算が走る
	c.runBackgroundMatching(1, "session-1")
	if got := stub.calls.Load(); got != 2 {
		t.Errorf("完了後に実行中フラグが残っている: 呼び出し回数=%d, want 2", got)
	}
}

// TestRunBackgroundMatching_DifferentSessionsRunConcurrently は
// 実行中フラグがセッション単位であることを検証する。
func TestRunBackgroundMatching_DifferentSessionsRunConcurrently(t *testing.T) {
	stub := &stubMatchingService{entered: make(chan struct{}, 2), block: make(chan struct{})}
	c := &ChatController{matchingService: stub}

	go c.runBackgroundMatching(1, "session-a")
	select {
	case <-stub.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("session-a が計算に入らなかった")
	}

	go c.runBackgroundMatching(2, "session-b")
	select {
	case <-stub.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("session-b が session-a に巻き込まれてブロックされた")
	}

	if got := stub.calls.Load(); got != 2 {
		t.Errorf("呼び出し回数=%d, want 2", got)
	}
	close(stub.block)
}

// TestRunBackgroundMatching_PassesDeadline は計算に打ち切り期限付きの ctx が
// 渡ることを検証する（context.Background() のままだと無期限に滞留する）。
func TestRunBackgroundMatching_PassesDeadline(t *testing.T) {
	stub := &stubMatchingService{}
	c := &ChatController{matchingService: stub}

	c.runBackgroundMatching(1, "session-deadline")

	stub.mu.Lock()
	ctx := stub.lastCtx
	stub.mu.Unlock()
	if ctx == nil {
		t.Fatal("CalculateMatching が呼ばれていない")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("期限の無い context が渡っている（打ち切りが効かない）")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > matchingRunTimeout {
		t.Errorf("期限が不正: remaining=%s, want (0, %s]", remaining, matchingRunTimeout)
	}
}
