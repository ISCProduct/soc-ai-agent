package maintenance

import (
	"context"
	"log/slog"
	"time"
)

const (
	// APICallLogRetentionMonths は生の api_call_logs を残す月数。
	// 管理画面の月次が最大24ヶ月なので、それより古い行は日次合計へ畳む（SOCAIAGENT-528）。
	APICallLogRetentionMonths = 24
	apiCallLogRetentionBatch  = 500
	// 1回の起動で消す上限。残りは翌日に回し、起動直後に DB を占有しない。
	apiCallLogRetentionMaxBatches = 50
	apiCallLogRetentionPause      = 100 * time.Millisecond
)

// APICallLogRetentionCutoff は、この時刻より古い呼び出しを日次合計へ畳む境界。
func APICallLogRetentionCutoff(now time.Time) time.Time {
	return now.UTC().AddDate(0, -APICallLogRetentionMonths, 0)
}

// APICallLogRollup は古い利用ログを1バッチだけ日次合計へ畳んで消す。
// ran が false のときは、他のプロセスが実行中なので何もしていない。
type APICallLogRollup interface {
	RollupBatch(ctx context.Context, cutoff time.Time, limit int) (deleted int, ran bool, err error)
}

// APICallLogRetainer は api_call_logs の24ヶ月より古い行を畳んで消す。
type APICallLogRetainer struct {
	repo       APICallLogRollup
	batchSize  int
	maxBatches int
	pause      time.Duration
	sleep      func(time.Duration)
	logger     *slog.Logger
}

// NewAPICallLogRetainer は日次ジョブから呼ぶ保持処理を作る。
func NewAPICallLogRetainer(repo APICallLogRollup, logger *slog.Logger) *APICallLogRetainer {
	if logger == nil {
		logger = slog.Default()
	}
	return &APICallLogRetainer{
		repo:       repo,
		batchSize:  apiCallLogRetentionBatch,
		maxBatches: apiCallLogRetentionMaxBatches,
		pause:      apiCallLogRetentionPause,
		sleep:      time.Sleep,
		logger:     logger,
	}
}

// RetentionResult は1回の実行で畳んで消した行数。
type RetentionResult struct {
	Deleted int
	// Skipped は他のプロセスが実行中で、この回は何もしなかったことを表す。
	Skipped bool
	// LimitReached は1回の上限まで消し、まだ古い行が残っている可能性があることを表す。
	LimitReached bool
	Err          error
}

// Run は cutoff より古い行を、バッチに分けて畳んで消す。退会では呼ばない。
func (p *APICallLogRetainer) Run(ctx context.Context, now time.Time) RetentionResult {
	cutoff := APICallLogRetentionCutoff(now)
	var result RetentionResult
	for i := range p.maxBatches {
		if i > 0 {
			p.sleep(p.pause)
		}
		n, ran, err := p.repo.RollupBatch(ctx, cutoff, p.batchSize)
		if !ran && err == nil {
			result.Skipped = true
			p.logger.Info("api_call_logs retention skipped; another run holds the lock")
			return result
		}
		result.Deleted += n
		if err != nil {
			result.Err = err
			p.logger.Error("api_call_logs retention failed", "deleted", result.Deleted, "error", err)
			return result
		}
		if n < p.batchSize {
			if result.Deleted > 0 {
				p.logger.Info("api_call_logs rolled up", "deleted", result.Deleted)
			}
			return result
		}
	}
	p.sleep(p.pause)
	n, ran, err := p.repo.RollupBatch(ctx, cutoff, 1)
	if err != nil {
		result.Err = err
		p.logger.Error("api_call_logs retention failed", "deleted", result.Deleted, "error", err)
		return result
	}
	if ran {
		result.Deleted += n
		result.LimitReached = n > 0
	}
	if result.LimitReached {
		p.logger.Warn("api_call_logs retention reached limit; the rest is left for the next run", "deleted", result.Deleted)
	} else if result.Deleted > 0 {
		p.logger.Info("api_call_logs rolled up", "deleted", result.Deleted)
	}
	return result
}
