package auth

import (
	"context"
	"testing"
	"time"

	"Backend/domain/repository"
	"Backend/internal/models"
)

// fakeGuestCleanupRepo は掃除に使う操作だけを持つ。ほかの操作が呼ばれたら nil 埋め込みで落ちる。
type fakeGuestCleanupRepo struct {
	repository.UserWithdrawalRepository
	staleIDs    []uint
	cutoff      time.Time
	limit       int
	withdrawn   map[uint]models.WithdrawnUser
	alreadyOut  map[uint]bool
	noLongerOld map[uint]bool // 一覧のあとで本登録・利用された（トランザクション内の確かめ直しで外れる）
	checkCutoff time.Time
}

func (f *fakeGuestCleanupRepo) ListStaleGuestIDs(_ context.Context, cutoff time.Time, limit int) ([]uint, error) {
	f.cutoff, f.limit = cutoff, limit
	return f.staleIDs, nil
}

func (f *fakeGuestCleanupRepo) WithdrawStaleGuest(_ context.Context, userID uint, cutoff, _ time.Time, build repository.WithdrawalRecordBuilder) error {
	f.checkCutoff = cutoff
	if f.alreadyOut[userID] {
		return repository.ErrAlreadyWithdrawn
	}
	if f.noLongerOld[userID] {
		return repository.ErrNotStaleGuest
	}
	f.withdrawn[userID] = build(&models.User{ID: userID, Email: "guest_synthetic@guest.invalid", IsGuest: true}, nil)
	return nil
}

// 最終発行から7日使われていないゲストを、退会と同じ経路で論理退会させ、データは12か月残す (SOCAIAGENT-689)。
// 一覧のあとで退会済み・本登録・利用されたゲストは数えず、退会もさせない（はらさんのレビュー）。
func TestUserDeletionService_ExpireStaleGuests(t *testing.T) {
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	repo := &fakeGuestCleanupRepo{
		staleIDs:    []uint{5, 6, 7, 8},
		withdrawn:   map[uint]models.WithdrawnUser{},
		alreadyOut:  map[uint]bool{6: true},
		noLongerOld: map[uint]bool{8: true},
	}
	svc := NewUserDeletionService(repo, nil, nil)

	n, err := svc.ExpireStaleGuests(context.Background(), now)
	if err != nil {
		t.Fatalf("ExpireStaleGuests: %v", err)
	}
	if n != 2 {
		t.Errorf("件数 = %d, want 2（退会済み・対象外になったゲストは数えない）", n)
	}
	want := now.Add(-7 * 24 * time.Hour)
	if !repo.cutoff.Equal(want) || !repo.checkCutoff.Equal(want) {
		t.Errorf("cutoff = %s / 確かめ直し %s, want 両方 %s", repo.cutoff, repo.checkCutoff, want)
	}
	if repo.limit <= 0 {
		t.Errorf("1回の件数に上限が必要: limit = %d", repo.limit)
	}
	if _, ok := repo.withdrawn[8]; ok {
		t.Error("確かめ直しで外れたゲストを退会させた")
	}
	for _, id := range []uint{5, 7} {
		rec, ok := repo.withdrawn[id]
		if !ok {
			t.Fatalf("user %d が退会処理されていない", id)
		}
		if rec.Reason != "guest_expired" {
			t.Errorf("reason = %q, want guest_expired", rec.Reason)
		}
		if !rec.WithdrawnAt.Equal(now) {
			t.Errorf("withdrawn_at = %s, want %s", rec.WithdrawnAt, now)
		}
	}
}

// 掃除で退会させたゲストのデータは12か月残す（2026-11-22 オーナー決定）。
// パージの対象は purge_after <= 実行時刻（ListDueWithdrawals の条件）なので、12か月ちょうどの前後で分かれる。
func TestUserDeletionService_ExpireStaleGuests_Retention12Months(t *testing.T) {
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	repo := &fakeGuestCleanupRepo{staleIDs: []uint{5}, withdrawn: map[uint]models.WithdrawnUser{}}
	if _, err := NewUserDeletionService(repo, nil, nil).ExpireStaleGuests(context.Background(), now); err != nil {
		t.Fatalf("ExpireStaleGuests: %v", err)
	}
	purgeAfter := repo.withdrawn[5].PurgeAfter
	twelveMonths := time.Date(2027, 11, 22, 3, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		at      time.Time
		wantDue bool
	}{
		{"退会直後は残す", now, false},
		{"12か月の1秒前は残す", twelveMonths.Add(-time.Second), false},
		{"12か月ちょうどで消す", twelveMonths, true},
		{"12か月の1秒後は消す", twelveMonths.Add(time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if due := !purgeAfter.After(tt.at); due != tt.wantDue {
				t.Errorf("purge_after = %s, %s 時点のパージ対象 = %v, want %v", purgeAfter, tt.at, due, tt.wantDue)
			}
		})
	}
}
