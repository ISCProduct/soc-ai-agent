package redisx

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewFromEnv は REDIS_URL からクライアントを生成する。未設定なら nil。
func NewFromEnv() *redis.Client {
	url := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if url == "" {
		log.Printf("[redis] REDIS_URL 未設定のためインメモリフォールバックを使用します")
		return nil
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("[redis] REDIS_URL のパースに失敗したためインメモリフォールバック: %v", err)
		return nil
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[redis] ping 失敗のためインメモリフォールバック: %v", err)
		_ = client.Close()
		return nil
	}
	log.Printf("[redis] connected (%s)", redactRedisURL(url))
	return client
}

func redactRedisURL(url string) string {
	// redis://:password@host:6379/0 → redis://***@host:6379/0
	if i := strings.Index(url, "@"); i >= 0 {
		schemeEnd := strings.Index(url, "://")
		if schemeEnd >= 0 {
			return url[:schemeEnd+3] + "***" + url[i:]
		}
	}
	return url
}
