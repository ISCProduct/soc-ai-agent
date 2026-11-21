package company

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"Backend/internal/models"
)

// claimFakeCrawlRepo は ClaimSource を DB の条件付き UPDATE と同じ条件で模したフェイク。
type claimFakeCrawlRepo struct {
	mu      sync.Mutex
	sources map[uint]*models.CrawlSource
	runs    []models.CrawlRun
	created int
	// listBarrier が非 nil なら、全実行体が一覧を取り終えるまで ListDueSources を返さない（競合を確実に起こす）
	listBarrier *sync.WaitGroup
}

func (r *claimFakeCrawlRepo) ListSources() ([]models.CrawlSource, error) { return nil, nil }
func (r *claimFakeCrawlRepo) GetSource(id uint) (*models.CrawlSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src, ok := r.sources[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *src
	return &cp, nil
}
func (r *claimFakeCrawlRepo) CreateSource(*models.CrawlSource) error { return nil }
func (r *claimFakeCrawlRepo) UpdateSource(source *models.CrawlSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *source
	r.sources[source.ID] = &cp
	return nil
}
func (r *claimFakeCrawlRepo) ListDueSources(now time.Time) ([]models.CrawlSource, error) {
	r.mu.Lock()
	var out []models.CrawlSource
	for _, s := range r.sources {
		if s.NextRunAt != nil && !s.NextRunAt.After(now) {
			out = append(out, *s)
		}
	}
	r.mu.Unlock()
	if r.listBarrier != nil {
		r.listBarrier.Done()
		r.listBarrier.Wait()
	}
	return out, nil
}
func (r *claimFakeCrawlRepo) ClaimSource(_ context.Context, id uint, now time.Time, lease time.Duration, onlyIfDue bool) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sources[id]
	if !ok {
		return false, nil
	}
	until := now.Add(lease)
	if onlyIfDue && (s.NextRunAt == nil || s.NextRunAt.After(now)) {
		return false, nil
	}
	if !onlyIfDue && s.NextRunAt != nil && s.NextRunAt.After(now) && !s.NextRunAt.After(until) {
		return false, nil
	}
	s.NextRunAt = &until
	return true, nil
}
func (r *claimFakeCrawlRepo) CreateRun(run *models.CrawlRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created++
	run.ID = uint(r.created)
	return nil
}
func (r *claimFakeCrawlRepo) UpdateRun(run *models.CrawlRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, *run)
	return nil
}
func (r *claimFakeCrawlRepo) ListRuns(uint, int) ([]models.CrawlRun, error) { return nil, nil }

func newClaimFake(src models.CrawlSource) *claimFakeCrawlRepo {
	return &claimFakeCrawlRepo{sources: map[uint]*models.CrawlSource{src.ID: &src}}
}

func TestRunDueSources_複数の実行体でも1回だけ実行される(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	repo := newClaimFake(models.CrawlSource{
		ID: 1, TargetType: "unsupported_for_test", ScheduleType: "daily", ScheduleTime: "02:00",
		IsActive: true, NextRunAt: &past,
	})
	repo.listBarrier = &sync.WaitGroup{}
	repo.listBarrier.Add(5)
	// 別々のサービス = 別々のタスク（プロセス内の mutex は効かない）
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewCrawlService(repo, nil, nil, nil).RunDueSources()
		}()
	}
	wg.Wait()
	if repo.created != 1 {
		t.Fatalf("CreateRun の回数 = %d, want 1", repo.created)
	}
}

func TestRunSource_実行中のソースは手動実行できない(t *testing.T) {
	leased := time.Now().Add(30 * time.Minute) // 確保済み（先送り時間内）
	repo := newClaimFake(models.CrawlSource{
		ID: 1, TargetType: "unsupported_for_test", ScheduleType: "daily", ScheduleTime: "02:00",
		IsActive: true, NextRunAt: &leased,
	})
	_, err := NewCrawlService(repo, nil, nil, nil).RunSource(1)
	if !errors.Is(err, ErrCrawlSourceBusy) {
		t.Fatalf("err = %v, want ErrCrawlSourceBusy", err)
	}
	if repo.created != 0 {
		t.Fatalf("CreateRun の回数 = %d, want 0", repo.created)
	}
}

func TestRunSource_先の予定なら手動実行できる(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	repo := newClaimFake(models.CrawlSource{
		ID: 1, TargetType: "unsupported_for_test", ScheduleType: "daily", ScheduleTime: "02:00",
		IsActive: true, NextRunAt: &future,
	})
	if _, err := NewCrawlService(repo, nil, nil, nil).RunSource(1); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.created != 1 {
		t.Fatalf("CreateRun の回数 = %d, want 1", repo.created)
	}
}

func TestRunSource_panicでもrunをfailedにし次回時刻を進める(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	// companyRepo が nil のため company クロールは panic する
	repo := newClaimFake(models.CrawlSource{
		ID: 1, Name: "x", TargetType: "company", ScheduleType: "daily", ScheduleTime: "02:00",
		IsActive: true, NextRunAt: &past,
	})
	run, err := NewCrawlService(repo, nil, nil, nil).RunSource(1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if run.Status != "failed" || run.EndedAt == nil {
		t.Fatalf("run = %+v, want failed で終了時刻あり", run)
	}
	got, _ := repo.GetSource(1)
	if got.NextRunAt == nil || got.NextRunAt.Sub(time.Now()) > 24*time.Hour || got.LastRunAt == nil {
		t.Fatalf("next_run_at = %v last_run_at = %v, want 次の定期時刻", got.NextRunAt, got.LastRunAt)
	}
}

func TestCrawlClaimLease(t *testing.T) {
	tests := []struct {
		env  string
		want time.Duration
	}{
		{"", 2 * time.Hour},
		{"3h", 3 * time.Hour},
		{"bad", 2 * time.Hour},
		{"-1h", 2 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("CRAWL_CLAIM_LEASE", tt.env)
			if got := crawlClaimLease(); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
