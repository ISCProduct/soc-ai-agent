package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// クライアント指定のリクエストIDはログとRAGへのヘッダーに流れるため、
// 形式が安全なものだけを受け入れる(#1188)。
func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		wantKept bool
	}{
		{name: "英数字とハイフンは採用", incoming: "req-123_ABC", wantKept: true},
		{name: "未指定なら採番", incoming: "", wantKept: false},
		{name: "空白入りは拒否", incoming: "req 123", wantKept: false},
		{name: "改行入りは拒否", incoming: "req\n123", wantKept: false},
		{name: "制御文字は拒否", incoming: "req\t123", wantKept: false},
		{name: "上限超過は拒否", incoming: strings.Repeat("a", maxRequestIDLen+1), wantKept: false},
		{name: "上限ちょうどは採用", incoming: strings.Repeat("a", maxRequestIDLen), wantKept: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fromCtx string
			h := RequestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				fromCtx = GetRequestID(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			if tt.incoming != "" {
				req.Header.Set(RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if tt.wantKept {
				if fromCtx != tt.incoming {
					t.Errorf("context requestID = %q, want %q", fromCtx, tt.incoming)
				}
			} else if fromCtx == tt.incoming || !isSafeRequestID(fromCtx) {
				t.Errorf("context requestID = %q, want a freshly generated safe id", fromCtx)
			}

			if got := rec.Header().Get(RequestIDHeader); got != fromCtx {
				t.Errorf("response header = %q, want %q", got, fromCtx)
			}
		})
	}
}
