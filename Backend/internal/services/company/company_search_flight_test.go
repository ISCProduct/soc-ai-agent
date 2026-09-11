package company

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompanySearchFlight_DoSingleflight(t *testing.T) {
	flight := NewCompanySearchFlight()
	var calls atomic.Int32
	var wg sync.WaitGroup
	const n = 8
	wg.Add(n)
	results := make([]any, n)
	errs := make([]error, n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = flight.Do("info", "acme", func() (any, error) {
				calls.Add(1)
				time.Sleep(30 * time.Millisecond)
				return "ok", nil
			})
		}(i)
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("fn calls=%d want 1", calls.Load())
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("err[%d]=%v", i, errs[i])
		}
		if results[i] != "ok" {
			t.Fatalf("result[%d]=%v", i, results[i])
		}
	}
}
