package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Setup は環境変数に基づいて slog をデフォルトロガーとして初期化する。
// LOG_LEVEL: DEBUG / INFO / WARN / ERROR (デフォルト: INFO)
// LOG_FORMAT: json / text (デフォルト: json)
//
// slog.SetDefault により既存の log.Println 等も slog 経由で出力される。
func Setup() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
