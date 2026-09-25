package safego

import (
	"sync"
	"testing"
	"time"
)

// panic した goroutine がプロセスを落とさず、後続の goroutine も動くことを固定する。
// Go() の defer recover を外すとこのテストはプロセスごと落ちる。
func TestGo_PanicDoesNotKillProcess(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	Go(func() {
		defer wg.Done()
		panic("boom")
	})

	ran := false
	Go(func() {
		defer wg.Done()
		ran = true
	})

	wg.Wait()
	if !ran {
		t.Fatal("panic した goroutine の後続が実行されていない")
	}
}

// Every は1回 panic しても周期実行を止めないことを固定する。
// 各回の recover を外すと2回目以降が走らず、ここで止まる。
// クロールや退会ユーザーの物理削除が再起動まで動かなくなるのを防ぐための性質。
func TestEvery_PanicDoesNotStopTicker(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	done := make(chan struct{})

	Every(time.Millisecond, true, func() {
		mu.Lock()
		calls++
		n := calls
		if n == 3 {
			close(done)
		}
		mu.Unlock()
		// 1回目だけ落とす。2回目以降が走ればチャネルが閉じる。
		if n == 1 {
			panic("boom")
		}
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		mu.Lock()
		n := calls
		mu.Unlock()
		t.Fatalf("panic 後に周期実行が止まった: 実行回数=%d", n)
	}
}

// runNow=false のとき、最初の tick を待ってから実行することを固定する。
func TestEvery_RunNowFalseWaitsForFirstTick(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	Every(200*time.Millisecond, false, func() {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	n := calls
	mu.Unlock()
	if n != 0 {
		t.Fatalf("runNow=false なのに tick 前に実行された: 実行回数=%d", n)
	}
}
