package controllers

// デバウンスマッチングのテスト（Issue #308）
// 実行: cd Backend && go test ./internal/controllers/... -run TestScheduleBackgroundMatching -v

import (
	"testing"
	"time"
)

// TestScheduleBackgroundMatching_CancelsOldTimer は同一セッションへの再呼び出しで
// 既存タイマーが置き換えられることを検証する（#308修正の担保）
func TestScheduleBackgroundMatching_CancelsOldTimer(t *testing.T) {
	c := &ChatController{}

	// 1回目のスケジュール
	c.scheduleBackgroundMatching(1, "session-1")
	first, ok := c.matchingTimers.Load("session-1")
	if !ok {
		t.Fatal("1回目のタイマーが登録されていない")
	}
	firstTimer := first.(*time.Timer)

	// 2回目のスケジュール（1回目はキャンセルされるべき）
	c.scheduleBackgroundMatching(1, "session-1")
	second, ok := c.matchingTimers.Load("session-1")
	if !ok {
		t.Fatal("2回目のタイマーが登録されていない")
	}
	secondTimer := second.(*time.Timer)

	// タイマーが置き換えられていること（異なるインスタンス）
	if firstTimer == secondTimer {
		t.Error("タイマーが置き換えられていない（デバウンスが機能していない）")
	}

	// クリーンアップ（タイマーが発火してnilサービスでpanicしないよう停止）
	secondTimer.Stop()
	c.matchingTimers.Delete("session-1")
}

// TestScheduleBackgroundMatching_DifferentSessionsAreIndependent は
// 異なるセッションのタイマーが互いに独立していることを検証する
func TestScheduleBackgroundMatching_DifferentSessionsAreIndependent(t *testing.T) {
	c := &ChatController{}

	c.scheduleBackgroundMatching(1, "session-a")
	c.scheduleBackgroundMatching(2, "session-b")

	timerA, okA := c.matchingTimers.Load("session-a")
	timerB, okB := c.matchingTimers.Load("session-b")

	if !okA {
		t.Error("session-a のタイマーが登録されていない")
	}
	if !okB {
		t.Error("session-b のタイマーが登録されていない")
	}
	if okA && okB && timerA.(*time.Timer) == timerB.(*time.Timer) {
		t.Error("異なるセッションに同一タイマーが登録されている")
	}

	// クリーンアップ
	if okA {
		timerA.(*time.Timer).Stop()
		c.matchingTimers.Delete("session-a")
	}
	if okB {
		timerB.(*time.Timer).Stop()
		c.matchingTimers.Delete("session-b")
	}
}

// TestScheduleBackgroundMatching_MultipleCallsReplaceTimer は
// N回連続で呼び出しても最終的に1つのタイマーのみ残ることを検証する（デバウンスの核心）
func TestScheduleBackgroundMatching_MultipleCallsReplaceTimer(t *testing.T) {
	c := &ChatController{}

	const n = 5
	for i := range n {
		c.scheduleBackgroundMatching(uint(i+1), "session-debounce")
	}

	// 1エントリのみ存在するはず
	count := 0
	c.matchingTimers.Range(func(k, v any) bool {
		if k.(string) == "session-debounce" {
			count++
		}
		return true
	})
	if count != 1 {
		t.Errorf("タイマー数が不正: got %d, want 1", count)
	}

	// クリーンアップ
	if t2, ok := c.matchingTimers.Load("session-debounce"); ok {
		t2.(*time.Timer).Stop()
		c.matchingTimers.Delete("session-debounce")
	}
}
