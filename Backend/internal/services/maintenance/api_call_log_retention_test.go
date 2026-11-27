package maintenance

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestAPICallLogRetentionCutoffIsTwentyFourMonths(t *testing.T) {
	now := time.Date(2026, 3, 31, 1, 2, 3, 0, time.UTC)
	got := APICallLogRetentionCutoff(now)
	want := time.Date(2024, 3, 31, 1, 2, 3, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("cutoff=%s want %s", got, want)
	}
	if !got.Before(now) {
		t.Fatal("境界が現在と同じか未来になっている")
	}
}

type fakeRollup struct {
	batches []int
	ran     bool
	calls   int
	cutoffs []time.Time
}

func (f *fakeRollup) RollupBatch(_ context.Context, cutoff time.Time, limit int) (int, bool, error) {
	f.calls++
	f.cutoffs = append(f.cutoffs, cutoff)
	if !f.ran {
		return 0, false, nil
	}
	n := limit
	if len(f.batches) > 0 {
		n = f.batches[0]
		f.batches = f.batches[1:]
	}
	return n, true, nil
}

func TestAPICallLogRetainerStopsWhenAnotherRunHoldsTheLock(t *testing.T) {
	repo := &fakeRollup{ran: false}
	p := NewAPICallLogRetainer(repo, slog.New(slog.DiscardHandler))
	p.sleep = func(time.Duration) {}
	got := p.Run(context.Background(), time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	if !got.Skipped || got.Deleted != 0 || repo.calls != 1 {
		t.Fatalf("skipped=%v deleted=%d calls=%d", got.Skipped, got.Deleted, repo.calls)
	}
}

func TestAPICallLogRetainerReportsWhenTheRunLimitIsHit(t *testing.T) {
	batches := make([]int, apiCallLogRetentionMaxBatches)
	for i := range batches {
		batches[i] = apiCallLogRetentionBatch
	}
	batches = append(batches, 1)
	repo := &fakeRollup{ran: true, batches: batches}
	p := NewAPICallLogRetainer(repo, slog.New(slog.DiscardHandler))
	p.sleep = func(time.Duration) {}
	got := p.Run(context.Background(), time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC))
	wantDeleted := apiCallLogRetentionMaxBatches*apiCallLogRetentionBatch + 1
	if !got.LimitReached || got.Deleted != wantDeleted || repo.calls != apiCallLogRetentionMaxBatches+1 {
		t.Fatalf("result=%+v calls=%d wantDeleted=%d", got, repo.calls, wantDeleted)
	}
}

func TestAPICallLogRetainerStopsAfterShortBatch(t *testing.T) {
	repo := &fakeRollup{ran: true, batches: []int{500, 3}}
	p := NewAPICallLogRetainer(repo, slog.New(slog.DiscardHandler))
	p.sleep = func(time.Duration) {}
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	got := p.Run(context.Background(), now)
	if got.Skipped || got.LimitReached || got.Deleted != 503 || repo.calls != 2 {
		t.Fatalf("result=%+v calls=%d", got, repo.calls)
	}
	want := APICallLogRetentionCutoff(now)
	for _, cutoff := range repo.cutoffs {
		if !cutoff.Equal(want) {
			t.Fatalf("cutoff=%s want %s", cutoff, want)
		}
	}
}
