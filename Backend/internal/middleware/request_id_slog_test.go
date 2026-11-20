package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRequestIDHandler_AddsIDFromContext(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(requestIDHandler{
		next: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx := WithRequestID(context.Background(), "req-1")
	slog.InfoContext(ctx, "register failed")
	slog.InfoContext(context.Background(), "no request")

	logText := buf.String()
	if !strings.Contains(logText, "request_id=req-1") {
		t.Fatalf("request_id が無い: %s", logText)
	}
	lines := strings.Split(strings.TrimSpace(logText), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, log = %s", len(lines), logText)
	}
	if strings.Contains(lines[1], "request_id=") {
		t.Fatalf("context の無いログに request_id が付いた: %s", lines[1])
	}
}
