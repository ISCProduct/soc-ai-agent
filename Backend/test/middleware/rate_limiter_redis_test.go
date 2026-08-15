package middleware_test

import (
	"testing"
	"time"

	"Backend/internal/middleware"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiter_BlocksOverLimit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	rl := middleware.NewRedisRateLimiter(rdb, "test", time.Minute, 3)
	for i := range 3 {
		if !rl.Allow("ip-a") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow("ip-a") {
		t.Fatal("4th request should be blocked")
	}
	if !rl.Allow("ip-b") {
		t.Fatal("different key should be independent")
	}
}

func TestRedisRateLimiter_NilClientFailOpen(t *testing.T) {
	rl := middleware.NewRedisRateLimiter(nil, "x", time.Minute, 1)
	if !rl.Allow("k") {
		t.Fatal("nil client should fail-open")
	}
}

func TestConfigureRateLimiters_SwapsGlobals(t *testing.T) {
	origLogin := middleware.LoginRateLimiter
	origReset := middleware.PasswordResetRateLimiter
	t.Cleanup(func() {
		middleware.ConfigureRateLimiters(origLogin, origReset)
	})

	custom := middleware.NewRateLimiter(time.Minute, 1)
	middleware.ConfigureRateLimiters(custom, custom)
	if !middleware.LoginRateLimiter.Allow("k") {
		t.Fatal("first should allow")
	}
	if middleware.LoginRateLimiter.Allow("k") {
		t.Fatal("second should block")
	}
}
