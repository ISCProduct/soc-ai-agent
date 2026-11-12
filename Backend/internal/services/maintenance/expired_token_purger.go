// Package maintenance は日次ジョブから呼ぶ掃除の処理をまとめる。
package maintenance

import (
	"log/slog"
	"time"
)

const (
	// purgeBatchSize は1文で消す件数の上限。1文が持つ行ロックの時間を短くするため（#2153）。
	purgeBatchSize = 1000
	// purgeMaxBatches は1回の実行で流すバッチ数の上限。1回あたり最大
	// purgeBatchSize × purgeMaxBatches 件まで消し、残りは翌日の実行に回す。
	// リフレッシュトークンはローテーションのたびに1行増えるので、1バッチだけだと
	// 増える速さに追いつかないおそれがある。
	purgeMaxBatches = 50
	// RefreshTokenRetention は期限切れのリフレッシュトークンを消すまでの猶予。
	// 期限が切れた直後の行を残し、直前の失効・ローテーションを監査で追えるようにする。
	RefreshTokenRetention = 7 * 24 * time.Hour
)

// ExpiredDeleter は cutoff で期限切れと判定した行を、最大 limit 件だけ消して件数を返す。
// PendingRegistrationRepository / UserRefreshTokenRepository の DeleteExpired が満たす。
type ExpiredDeleter interface {
	DeleteExpired(cutoff time.Time, limit int) (int64, error)
}

type purgeTarget struct {
	name    string
	deleter ExpiredDeleter
	// cutoff は実行時刻から、DeleteExpired に渡す時刻を求める。
	cutoff func(now time.Time) time.Time
}

// PurgeResult は1つの対象の削除結果。
type PurgeResult struct {
	Target  string
	Deleted int64
	// LimitReached は1回の上限まで消して、まだ残っている可能性があることを表す。
	LimitReached bool
	Err          error
}

// ExpiredTokenPurger は期限切れの仮登録とリフレッシュトークンを消す（#2153）。
// 仮登録にはメールアドレスが入っており、溜め続けると不要な個人情報を持ち続けることになる。
type ExpiredTokenPurger struct {
	targets    []purgeTarget
	batchSize  int
	maxBatches int
	logger     *slog.Logger
}

// NewExpiredTokenPurger は仮登録・リフレッシュトークンの順に消す ExpiredTokenPurger を作る。
func NewExpiredTokenPurger(pendingRegistrations, refreshTokens ExpiredDeleter, logger *slog.Logger) *ExpiredTokenPurger {
	return &ExpiredTokenPurger{
		targets: []purgeTarget{
			{
				name:    "pending_registrations",
				deleter: pendingRegistrations,
				// 仮登録は期限が切れた時点で使えず、監査で追う用途も無いので猶予を置かない。
				cutoff: func(now time.Time) time.Time { return now },
			},
			{
				name:    "user_refresh_tokens",
				deleter: refreshTokens,
				cutoff:  func(now time.Time) time.Time { return now.Add(-RefreshTokenRetention) },
			},
		},
		batchSize:  purgeBatchSize,
		maxBatches: purgeMaxBatches,
		logger:     logger,
	}
}

// Run は各対象の期限切れの行を消し、結果を対象の順に返す。
//
// 1つの対象で失敗しても、ログに残して次の対象に進む（退会ユーザーのパージと同じ扱い）。
// 失敗した対象の残りは翌日の実行で消し直す。
// ログに出すのは対象のテーブル名・件数・エラーだけで、メールアドレスやトークンは出さない。
func (p *ExpiredTokenPurger) Run(now time.Time) []PurgeResult {
	results := make([]PurgeResult, 0, len(p.targets))
	for _, t := range p.targets {
		r := p.purge(t, now)
		results = append(results, r)
		switch {
		case r.Err != nil:
			// 失敗する前のバッチで消えた分も、件数として残す
			p.logger.Error("purge expired tokens failed", "target", r.Target, "deleted", r.Deleted, "error", r.Err)
		case r.LimitReached:
			// 上限に当たり続けるなら上限の見直しが要るので、毎回出す
			p.logger.Warn("purge expired tokens reached limit; the rest is left for the next run", "target", r.Target, "deleted", r.Deleted)
		case r.Deleted > 0:
			p.logger.Info("purged expired tokens", "target", r.Target, "deleted", r.Deleted)
		}
	}
	return results
}

// purge は1つの対象をバッチに分けて消す。
// 1バッチが上限より少なければ、消し終えたとみなして止める。
// 複数台で同時に走ると、もう1台が消した分だけ少なく返ることがあるが、
// 残りはそちらの実行か翌日の実行で消えるので問題ない。
func (p *ExpiredTokenPurger) purge(t purgeTarget, now time.Time) PurgeResult {
	cutoff := t.cutoff(now)
	r := PurgeResult{Target: t.name}
	for range p.maxBatches {
		n, err := t.deleter.DeleteExpired(cutoff, p.batchSize)
		r.Deleted += n
		if err != nil {
			r.Err = err
			return r
		}
		if n < int64(p.batchSize) {
			return r
		}
	}
	r.LimitReached = true
	return r
}
