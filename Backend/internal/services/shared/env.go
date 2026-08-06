package shared

import (
	"os"
	"strconv"
	"strings"
)

// GetEnv は環境変数を取得し、未設定・空文字ならデフォルト値を返す。
func GetEnv(key, def string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// GetIntEnv は環境変数を int として取得し、未設定・不正値ならデフォルト値を返す。
func GetIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetFloatEnv は環境変数を float64 として取得し、未設定・不正値ならデフォルト値を返す。
func GetFloatEnv(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}
