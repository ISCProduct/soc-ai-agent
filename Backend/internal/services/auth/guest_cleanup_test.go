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
	staleIDs   []uint
	cutoff     time.Time
	limit      int
	withdrawn  map[uint]models.WithdrawnUser
	alreadyOut map[uint]bool
}

func (f *fakeGuestCleanupRepo) ListStaleGuestIDs(_ context.Context, cutoff time.Time, limit int) ([]uint, error) {
	f.cutoff, f.limit = cutoff, limit
	return f.staleIDs, nil
}

func (f *fakeGuestCleanupRepo) Withdraw(userID uint, _ time.Time, build repository.WithdrawalRecordBuilder) error {
	if f.alreadyOut[userID] {
		return repository.ErrAlreadyWithdrawn
	}
	f.withdrawn[userID] = build(&models.User{ID: userID, Email: "guest_synthetic@guest.invalid", IsGuest: true}, nil)
	return nil
}

// 最終発行から7日使われていないゲストを、退会と同じ経路で、保持期間なし（すぐパージ対象）にする (SOCAIAGENT-689)。
func TestUserDeletionService_ExpireStaleGuests(t *testing.T) {
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	repo := &fakeGuestCleanupRepo{
		staleIDs:   []uint{5, 6, 7},
		withdrawn:  map[uint]models.WithdrawnUser{},
		alreadyOut: map[uint]bool{6: true},
	}
	svc := NewUserDeletionService(repo, nil, nil)

	n, err := svc.ExpireStaleGuests(context.Background(), now)
	if err != nil {
		t.Fatalf("ExpireStaleGuests: %v", err)
	}
	if n != 2 {
		t.Errorf("件数 = %d, want 2（退会済みは数えない）", n)
	}
	if want := now.Add(-7 * 24 * time.Hour); !repo.cutoff.Equal(want) {
		t.Errorf("cutoff = %s, want %s", repo.cutoff, want)
	}
	if repo.limit <= 0 {
		t.Errorf("1回の件数に上限が必要: limit = %d", repo.limit)
	}
	for _, id := range []uint{5, 7} {
		rec, ok := repo.withdrawn[id]
		if !ok {
			t.Fatalf("user %d が退会処理されていない", id)
		}
		if rec.Reason != "guest_expired" {
			t.Errorf("reason = %q, want guest_expired", rec.Reason)
		}
		if !rec.PurgeAfter.Equal(now) || !rec.WithdrawnAt.Equal(now) {
			t.Errorf("purge_after = %s withdrawn_at = %s, want 両方 %s（保持期間なし）", rec.PurgeAfter, rec.WithdrawnAt, now)
		}
	}
}
