package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// sliding window を原子的に判定する Lua スクリプト（#617）
var redisRateLimitScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
local n = redis.call('ZCARD', KEYS[1])
if n >= tonumber(ARGV[2]) then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[4])
redis.call('EXPIRE', KEYS[1], ARGV[5])
return 1
`)

// RedisRateLimiter は Redis ZSET によるスライディングウィンドウ制限（#617）。
type RedisRateLimiter struct {
	client  *redis.Client
	prefix  string
	window  time.Duration
	maxReqs int
}

// NewRedisRateLimiter は Redis ベースの KeyRateLimiter を生成する。
func NewRedisRateLimiter(client *redis.Client, prefix string, window time.Duration, maxReqs int) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:  client,
		prefix:  prefix,
		window:  window,
		maxReqs: maxReqs,
	}
}

// Allow はキーに対するリクエストを記録し、制限内なら true を返す。
// Redis 障害時はフェイルオープン（true）。
func (rl *RedisRateLimiter) Allow(key string) bool {
	if rl == nil || rl.client == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	now := time.Now()
	redisKey := fmt.Sprintf("ratelimit:%s:%s", rl.prefix, key)
	cutoff := strconv.FormatInt(now.Add(-rl.window).UnixNano(), 10)
	score := strconv.FormatInt(now.UnixNano(), 10)
	member := score
	expireSec := int64(rl.window.Seconds()) + 60

	res, err := redisRateLimitScript.Run(ctx, rl.client, []string{redisKey},
		cutoff, rl.maxReqs, score, member, expireSec,
	).Int()
	if err != nil {
		return true
	}
	return res == 1
}
