package safego

import (
	"sync"
	"testing"
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
