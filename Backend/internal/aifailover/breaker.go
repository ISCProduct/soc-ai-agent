package aifailover

import (
	"sync"
	"time"
)

// Breaker は1つの工程（STT / LLM / TTS）の Circuit Breaker。
//
// 縮退してよい失敗が threshold 回続くと開き、cooldown の間は OpenAI を呼ばずにローカルへ
// 直接回す。障害中に毎回 OpenAI のタイムアウトを待つと、面接の1ターンが数十秒止まるため。
// cooldown が過ぎたら1回だけ OpenAI を試し（半開）、成功すれば閉じ、失敗すればまた開く。
type Breaker struct {
	threshold int
	cooldown  time.Duration
	now       func() time.Time

	mu        sync.Mutex
	failures  int
	openUntil time.Time
	probing   bool
}

// NewBreaker は Breaker を作る。threshold が 1 未満なら 1 にする。
func NewBreaker(threshold int, cooldown time.Duration) *Breaker {
	if threshold < 1 {
		threshold = 1
	}
	return &Breaker{threshold: threshold, cooldown: cooldown, now: time.Now}
}

// Allow は OpenAI を呼んでよいかを返す。開いている間は false。cooldown が過ぎたら
// 1つの呼び出しにだけ true を返す（半開の試し）。
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures < b.threshold {
		return true
	}
	if b.now().Before(b.openUntil) || b.probing {
		return false
	}
	b.probing = true
	return true
}

// Success は OpenAI の呼び出しが成功したことを記録し、閉じる。
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.probing = false
	b.openUntil = time.Time{}
}

// Failure は縮退してよい失敗を記録する。threshold に達したら cooldown の間開く。
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.probing = false
	if b.failures >= b.threshold {
		b.openUntil = b.now().Add(b.cooldown)
	}
}

// Open は今開いているか（OpenAI を呼ばない状態か）を返す。ログと状態の表示用。
func (b *Breaker) Open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failures >= b.threshold && b.now().Before(b.openUntil)
}
