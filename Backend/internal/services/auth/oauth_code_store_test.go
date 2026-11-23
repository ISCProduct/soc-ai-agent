package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func bindHashOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// メモリ版と Redis 版で同じ決まりを守ることを確かめる。
func codeStores(t *testing.T) map[string]*OAuthCodeStore {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return map[string]*OAuthCodeStore{
		"memory": NewOAuthCodeStore(nil),
		"redis":  NewOAuthCodeStore(rdb),
	}
}

func TestOAuthCodeStore_Redeem(t *testing.T) {
	const verifier = "verifier-of-this-browser"
	cases := []struct {
		name     string
		bindHash string
		redeemAs string
		wantErr  error
	}{
		{"照合値が合えば中身を返す", bindHashOf(verifier), verifier, nil},
		{"照合値が違えば拒否する", bindHashOf(verifier), "another-browser", ErrOAuthBindMismatch},
		{"照合値なしで発行されたコードは拒否する", "", verifier, ErrOAuthBindMismatch},
	}
	for storeName, store := range codeStores(t) {
		for _, tt := range cases {
			t.Run(storeName+"/"+tt.name, func(t *testing.T) {
				ctx := context.Background()
				code, err := store.Issue(ctx, []byte(`{"user_token":"jwt"}`), tt.bindHash)
				if err != nil {
					t.Fatalf("Issue: %v", err)
				}
				payload, err := store.Redeem(ctx, code, tt.redeemAs)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if tt.wantErr == nil && string(payload) != `{"user_token":"jwt"}` {
					t.Fatalf("payload = %q", payload)
				}
			})
		}
	}
}

func TestOAuthCodeStore_SingleUse(t *testing.T) {
	const verifier = "v"
	for storeName, store := range codeStores(t) {
		t.Run(storeName+"/使ったコードは二度と使えない", func(t *testing.T) {
			ctx := context.Background()
			code, _ := store.Issue(ctx, []byte("p"), bindHashOf(verifier))
			if _, err := store.Redeem(ctx, code, verifier); err != nil {
				t.Fatalf("1回目: %v", err)
			}
			if _, err := store.Redeem(ctx, code, verifier); !errors.Is(err, ErrOAuthCodeInvalid) {
				t.Fatalf("2回目 err = %v, want ErrOAuthCodeInvalid", err)
			}
		})
		t.Run(storeName+"/照合値を間違えたコードも消える", func(t *testing.T) {
			ctx := context.Background()
			code, _ := store.Issue(ctx, []byte("p"), bindHashOf(verifier))
			_, _ = store.Redeem(ctx, code, "wrong")
			if _, err := store.Redeem(ctx, code, verifier); !errors.Is(err, ErrOAuthCodeInvalid) {
				t.Fatalf("err = %v, want ErrOAuthCodeInvalid", err)
			}
		})
		t.Run(storeName+"/存在しないコードは拒否する", func(t *testing.T) {
			if _, err := store.Redeem(context.Background(), "no-such-code", verifier); !errors.Is(err, ErrOAuthCodeInvalid) {
				t.Fatalf("err = %v, want ErrOAuthCodeInvalid", err)
			}
		})
	}
}

func TestOAuthCodeStore_Expiry(t *testing.T) {
	const verifier = "v"
	t.Run("memory/期限切れは拒否する", func(t *testing.T) {
		store := NewOAuthCodeStore(nil)
		now := time.Now()
		store.now = func() time.Time { return now }
		code, _ := store.Issue(context.Background(), []byte("p"), bindHashOf(verifier))
		now = now.Add(OAuthCodeTTL + time.Second)
		if _, err := store.Redeem(context.Background(), code, verifier); !errors.Is(err, ErrOAuthCodeInvalid) {
			t.Fatalf("err = %v, want ErrOAuthCodeInvalid", err)
		}
	})
	t.Run("redis/期限切れは拒否する", func(t *testing.T) {
		mr := miniredis.RunT(t)
		rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer rdb.Close()
		store := NewOAuthCodeStore(rdb)
		code, _ := store.Issue(context.Background(), []byte("p"), bindHashOf(verifier))
		mr.FastForward(OAuthCodeTTL + time.Second)
		if _, err := store.Redeem(context.Background(), code, verifier); !errors.Is(err, ErrOAuthCodeInvalid) {
			t.Fatalf("err = %v, want ErrOAuthCodeInvalid", err)
		}
	})
}

func TestOAuthCodeStore_CodesAreUnique(t *testing.T) {
	store := NewOAuthCodeStore(nil)
	seen := map[string]bool{}
	for range 100 {
		code, err := store.Issue(context.Background(), []byte("p"), "h")
		if err != nil {
			t.Fatal(err)
		}
		if len(code) < 43 || seen[code] {
			t.Fatalf("推測しにくい一意なコードではない: %q", code)
		}
		seen[code] = true
	}
}
