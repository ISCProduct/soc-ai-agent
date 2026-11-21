package middleware

import (
	"context"
	"log/slog"
)

// requestIDHandler は slog の context に載っているリクエストIDをログ属性へ足す。
// log.Printf は context を持たないので、ここには乗らない。
type requestIDHandler struct {
	next slog.Handler
}

func (h requestIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if rid := GetRequestID(ctx); rid != "" {
		r.AddAttrs(slog.String("request_id", rid))
	}
	return h.next.Handle(ctx, r)
}

func (h requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestIDHandler{next: h.next.WithAttrs(attrs)}
}

func (h requestIDHandler) WithGroup(name string) slog.Handler {
	return requestIDHandler{next: h.next.WithGroup(name)}
}

// UseRequestIDLogger は既定の slog にリクエストIDを載せる。
// logger.Setup のあとに1回呼ぶ。
func UseRequestIDLogger() {
	slog.SetDefault(slog.New(requestIDHandler{next: slog.Default().Handler()}))
}
