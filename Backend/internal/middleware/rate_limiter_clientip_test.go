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

// TestGetClientIP_BFFForwarded は BFF(Next.js Route Handler)が転送した実クライアントIPを
// 「信頼できる経路から来た場合だけ」採用することを検証する (#1407)。
//
// backend の ALB はインターネット直結なので X-Client-IP は誰でも送れる。
// BFF と共有する BFF_INTERNAL_TOKEN が一致したときに限り採用し、それ以外は
// 従来どおり ALB が付けた XFF 末尾へフォールバックしなければならない。
func TestGetClientIP_BFFForwarded(t *testing.T) {
	tests := []struct {
		name        string
		envToken    string
		headerToken string
		clientIP    string
		xff         string
		remoteAddr  string
		want        string
	}{
		{
			name:       "トークン未設定: X-Client-IPは無視してXFF末尾",
			envToken:   "",
			clientIP:   "198.51.100.7",
			xff:        "1.1.1.1, 203.0.113.5",
			remoteAddr: "10.0.0.1:1234",
			want:       "203.0.113.5",
		},
		{
			name:        "トークン不一致: X-Client-IPは無視してXFF末尾",
			envToken:    "secret-token",
			headerToken: "wrong-token",
			clientIP:    "198.51.100.7",
			xff:         "1.1.1.1, 203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "203.0.113.5",
		},
		{
			name:       "トークンヘッダー無し: X-Client-IPは無視してXFF末尾",
			envToken:   "secret-token",
			clientIP:   "198.51.100.7",
			xff:        "1.1.1.1, 203.0.113.5",
			remoteAddr: "10.0.0.1:1234",
			want:       "203.0.113.5",
		},
		{
			name:        "トークン一致: X-Client-IPを採用",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "198.51.100.7",
			xff:         "1.1.1.1, 203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "198.51.100.7",
		},
		{
			name:        "トークン一致(IPv6): X-Client-IPを採用",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "2001:db8::1",
			xff:         "203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "2001:db8::1",
		},
		{
			name:        "トークン一致(ポート付き): ホスト部のみ採用",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "198.51.100.7:51234",
			xff:         "203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "198.51.100.7",
		},
		{
			name:        "トークン一致だがIPとして不正: XFF末尾へフォールバック",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "not-an-ip",
			xff:         "203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "203.0.113.5",
		},
		{
			name:        "トークン一致だがX-Client-IP空: XFF末尾へフォールバック",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "",
			xff:         "203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "203.0.113.5",
		},
		{
			name:        "X-Client-IPも多段連結されていたら末尾を採用",
			envToken:    "secret-token",
			headerToken: "secret-token",
			clientIP:    "1.1.1.1, 198.51.100.7",
			xff:         "203.0.113.5",
			remoteAddr:  "10.0.0.1:1234",
			want:        "198.51.100.7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(BFFInternalTokenEnv, tt.envToken)
			r, err := http.NewRequest(http.MethodPost, "/", nil)
			if err != nil {
				t.Fatalf("リクエスト生成に失敗: %v", err)
			}
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.headerToken != "" {
				r.Header.Set(InternalTokenHeader, tt.headerToken)
			}
			if tt.clientIP != "" {
				r.Header.Set(ClientIPHeader, tt.clientIP)
			}
			if got := GetClientIP(r); got != tt.want {
				t.Errorf("GetClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRateLimiterPerClientIPViaBFF は BFF 経由でも利用者ごとに別キーで
// 制限されることを、レート制限器と結合して検証する (#1407)。
// 転送前は BFF の出口IP1つに収束し、全利用者合計で上限に当たっていた。
func TestRateLimiterPerClientIPViaBFF(t *testing.T) {
	t.Setenv(BFFInternalTokenEnv, "secret-token")
	limiter := NewRateLimiter(time.Minute, 3)

	// 別々の利用者が BFF 経由で1回ずつ叩く。上限3回でも全員通るべき。
	clients := []string{"198.51.100.1", "198.51.100.2", "198.51.100.3", "198.51.100.4", "198.51.100.5"}
	allowed := 0
	for _, client := range clients {
		r, err := http.NewRequest(http.MethodPost, "/", nil)
		if err != nil {
			t.Fatalf("リクエスト生成に失敗: %v", err)
		}
		r.RemoteAddr = "10.0.0.1:1234"
		// BFF -> api ALB の1ホップ。ALB が末尾に BFF タスクの出口IPを追記する。
		r.Header.Set("X-Forwarded-For", "203.0.113.5")
		r.Header.Set(InternalTokenHeader, "secret-token")
		r.Header.Set(ClientIPHeader, client)
		if limiter.Allow(GetClientIP(r)) {
			allowed++
		}
	}

	if allowed != len(clients) {
		t.Errorf("利用者 %d人のうち許可されたのは %d人。IP単位の制限が全体の上限として効いている", len(clients), allowed)
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
