package middleware

import (
	"net/http"
	"testing"
	"time"
)

// TestGetClientIP は X-Forwarded-For の詐称でレート制限のキーを分散できないことを検証する。
// ALB は既存の XFF の末尾へ実IPを追記するため、信頼できるのは最後の要素のみ。
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xRealIP    string
		remoteAddr string
		want       string
	}{
		{"XFFなし: RemoteAddrから取得", "", "", "203.0.113.5:1234", "203.0.113.5"},
		{"XFF単一: そのまま", "203.0.113.5", "", "10.0.0.1:1234", "203.0.113.5"},
		{"XFF詐称: 末尾の実IPを採用", "1.1.1.1, 203.0.113.5", "", "10.0.0.1:1234", "203.0.113.5"},
		{"XFF詐称(別値): 同じキーになる", "2.2.2.2, 203.0.113.5", "", "10.0.0.1:1234", "203.0.113.5"},
		{"XFF詐称(多段): 末尾のみ採用", "1.1.1.1, 2.2.2.2, 203.0.113.5", "", "10.0.0.1:1234", "203.0.113.5"},
		{"XFF末尾にポート付き: ホスト部のみ", "1.1.1.1, 203.0.113.5:443", "", "10.0.0.1:1234", "203.0.113.5"},
		{"空白のゆらぎを吸収", "1.1.1.1,   203.0.113.5  ", "", "10.0.0.1:1234", "203.0.113.5"},
		{"XFF不在時はX-Real-IP", "", "203.0.113.9", "10.0.0.1:1234", "203.0.113.9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodPost, "/", nil)
			if err != nil {
				t.Fatalf("リクエスト生成に失敗: %v", err)
			}
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				r.Header.Set("X-Real-IP", tt.xRealIP)
			}
			if got := GetClientIP(r); got != tt.want {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRateLimiterNotBypassableBySpoofedXFF は XFF の先頭を毎回変えても
// 同一クライアントとして制限されることを、レート制限器と結合して検証する。
func TestRateLimiterNotBypassableBySpoofedXFF(t *testing.T) {
	limiter := NewRateLimiter(time.Minute, 3)
	spoofs := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}

	allowed := 0
	for _, spoof := range spoofs {
		r, err := http.NewRequest(http.MethodPost, "/", nil)
		if err != nil {
			t.Fatalf("リクエスト生成に失敗: %v", err)
		}
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set("X-Forwarded-For", spoof+", 203.0.113.5")
		if limiter.Allow(GetClientIP(r)) {
			allowed++
		}
	}

	if allowed != 3 {
		t.Errorf("詐称XFF %d件のうち許可されたのは %d件。上限3件で頭打ちになるべき（レート制限が回避されている）", len(spoofs), allowed)
	}
}
