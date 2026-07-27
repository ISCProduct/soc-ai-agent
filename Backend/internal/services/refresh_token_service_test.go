package services

import (
	"testing"
	"time"

	"Backend/internal/models"
)

// fakeRefreshTokenStore はテスト用のインメモリストア
type fakeRefreshTokenStore struct {
	tokens map[string]*models.UserRefreshToken
	nextID uint
}

func newFakeRefreshTokenStore() *fakeRefreshTokenStore {
	return &fakeRefreshTokenStore{tokens: map[string]*models.UserRefreshToken{}}
}

func (f *fakeRefreshTokenStore) Create(token *models.UserRefreshToken) error {
	f.nextID++
	token.ID = f.nextID
	f.tokens[token.TokenHash] = token
	return nil
}

func (f *fakeRefreshTokenStore) FindByHash(hash string) (*models.UserRefreshToken, error) {
	if t, ok := f.tokens[hash]; ok {
		copied := *t
		return &copied, nil
	}
	return nil, nil
}

func (f *fakeRefreshTokenStore) Revoke(id uint, at time.Time) error {
	for _, t := range f.tokens {
		if t.ID == id && t.RevokedAt == nil {
			t.RevokedAt = &at
		}
	}
	return nil
}

func (f *fakeRefreshTokenStore) RevokeAllByUser(userID uint, at time.Time) error {
	for _, t := range f.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &at
		}
	}
	return nil
}

func newTestService(store *fakeRefreshTokenStore, now time.Time) *RefreshTokenService {
	s := NewRefreshTokenService(store)
	s.now = func() time.Time { return now }
	return s
}

func TestRefreshTokenService_IssueAndRotate(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	t.Run("発行したトークンでローテーションできる", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)

		plain, err := svc.Issue(42)
		if err != nil {
			t.Fatalf("Issue に失敗: %v", err)
		}
		if len(plain) != 64 {
			t.Errorf("トークン長 = %d, want 64", len(plain))
		}

		userID, newPlain, err := svc.Rotate(plain)
		if err != nil {
			t.Fatalf("Rotate に失敗: %v", err)
		}
		if userID != 42 {
			t.Errorf("userID = %d, want 42", userID)
		}
		if newPlain == plain {
			t.Error("ローテーション後のトークンが変わっていない")
		}
	})

	t.Run("ローテーション後の旧トークンは猶予期間内なら再利用できる", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)

		plain, _ := svc.Issue(1)
		if _, _, err := svc.Rotate(plain); err != nil {
			t.Fatalf("1回目の Rotate に失敗: %v", err)
		}
		// 猶予期間内（30秒後）の並行リフレッシュを想定
		svc.now = func() time.Time { return now.Add(30 * time.Second) }
		if _, _, err := svc.Rotate(plain); err != nil {
			t.Errorf("猶予期間内の再利用が拒否された: %v", err)
		}
	})

	t.Run("猶予期間を過ぎた失効トークンは拒否される", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)

		plain, _ := svc.Issue(1)
		if _, _, err := svc.Rotate(plain); err != nil {
			t.Fatalf("Rotate に失敗: %v", err)
		}
		svc.now = func() time.Time { return now.Add(2 * time.Minute) }
		if _, _, err := svc.Rotate(plain); err != ErrInvalidRefreshToken {
			t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
		}
	})
}

func TestRefreshTokenService_Validate(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func(svc *RefreshTokenService) string // 検証対象トークンを返す
		atTime  time.Time
		wantErr error
	}{
		{
			name:    "空文字は無効",
			setup:   func(svc *RefreshTokenService) string { return "" },
			atTime:  now,
			wantErr: ErrInvalidRefreshToken,
		},
		{
			name:    "存在しないトークンは無効",
			setup:   func(svc *RefreshTokenService) string { return "unknown-token" },
			atTime:  now,
			wantErr: ErrInvalidRefreshToken,
		},
		{
			name: "期限切れトークンは無効",
			setup: func(svc *RefreshTokenService) string {
				plain, _ := svc.Issue(1)
				return plain
			},
			atTime:  now.Add(refreshTokenTTL + time.Hour),
			wantErr: ErrInvalidRefreshToken,
		},
		{
			name: "有効なトークンは通る",
			setup: func(svc *RefreshTokenService) string {
				plain, _ := svc.Issue(1)
				return plain
			},
			atTime:  now.Add(29 * 24 * time.Hour),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeRefreshTokenStore()
			svc := newTestService(store, now)
			plain := tt.setup(svc)

			svc.now = func() time.Time { return tt.atTime }
			_, err := svc.validate(plain)
			if err != tt.wantErr {
				t.Errorf("validate() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRefreshTokenService_Revoke(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	t.Run("Revoke後のトークンは猶予期間を過ぎると使えない", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)

		plain, _ := svc.Issue(1)
		if err := svc.Revoke(plain); err != nil {
			t.Fatalf("Revoke に失敗: %v", err)
		}
		svc.now = func() time.Time { return now.Add(2 * time.Minute) }
		if _, _, err := svc.Rotate(plain); err != ErrInvalidRefreshToken {
			t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
		}
	})

	t.Run("RevokeAllForUserで全トークンが失効する", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)

		t1, _ := svc.Issue(1)
		t2, _ := svc.Issue(1)
		other, _ := svc.Issue(2)

		if err := svc.RevokeAllForUser(1); err != nil {
			t.Fatalf("RevokeAllForUser に失敗: %v", err)
		}
		svc.now = func() time.Time { return now.Add(2 * time.Minute) }
		for _, plain := range []string{t1, t2} {
			if _, _, err := svc.Rotate(plain); err != ErrInvalidRefreshToken {
				t.Errorf("user1 のトークンが失効していない: %v", err)
			}
		}
		if _, _, err := svc.Rotate(other); err != nil {
			t.Errorf("別ユーザーのトークンまで失効している: %v", err)
		}
	})

	t.Run("不明なトークンのRevokeはエラーにしない", func(t *testing.T) {
		store := newFakeRefreshTokenStore()
		svc := newTestService(store, now)
		if err := svc.Revoke("unknown"); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
}

func TestHashRefreshToken(t *testing.T) {
	first := hashRefreshToken("abc")
	second := hashRefreshToken("abc")
	if first != second {
		t.Error("同一入力でハッシュが一致しない")
	}
	if first == hashRefreshToken("abd") {
		t.Error("異なる入力でハッシュが衝突している")
	}
	if len(first) != 64 {
		t.Errorf("ハッシュ長 = %d, want 64 (SHA-256 hex)", len(first))
	}
}
