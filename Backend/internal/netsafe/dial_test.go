package netsafe

import (
	"context"
	"errors"
	"net"
	"testing"
)

// TestDialContext_BlocksInternalAddresses は接続直前のIP検査が内部アドレスを
// 拒否することを固定する（#940 / #1408）。
//
// 実際に外部へ接続する成功系はユニットテストにしない（ネットワーク依存になるため）。
func TestDialContext_BlocksInternalAddresses(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		mockIPs []net.IP
		mockErr error
	}{
		{name: "リテラル内部IPへのdialはブロックされる", addr: "127.0.0.1:80"},
		{name: "プライベートIPリテラル", addr: "10.20.1.85:3306"},
		{
			name:    "内部IPに解決されるホスト名(DNSリバインディング対策)",
			addr:    "internal.example.com:80",
			mockIPs: []net.IP{net.ParseIP("169.254.169.254")},
		},
		{
			// 公開IPと内部IPの両方を返す応答で内部側を引かせる手を塞ぐ
			name:    "公開IPと内部IPが混在するホスト名",
			addr:    "mixed.example.com:80",
			mockIPs: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("169.254.169.254")},
		},
		{
			name:    "名前解決に失敗した場合はブロックされる",
			addr:    "unresolvable.example.com:80",
			mockErr: errors.New("no such host"),
		},
		{
			name:    "解決結果が空でもブロックされる",
			addr:    "empty.example.com:80",
			mockIPs: []net.IP{},
		},
	}

	original := LookupIP
	defer func() { LookupIP = original }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LookupIP = func(string) ([]net.IP, error) { return tt.mockIPs, tt.mockErr }
			if _, err := DialContext(context.Background(), "tcp", tt.addr); err == nil {
				t.Errorf("内部アドレスへのdialはエラーになるべき: addr=%s", tt.addr)
			}
		})
	}
}

// TestIsInternalIP は塞ぐべきアドレス範囲を固定する。
// とくに 169.254.169.254 はクラウドのメタデータエンドポイントで、
// 抜けるとインスタンスの資格情報が読まれる。
func TestIsInternalIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // メタデータエンドポイント
		{"0.0.0.0", true},
		{"fc00::1", true}, // IPv6 ユニークローカル
		{"fe80::1", true}, // IPv6 リンクローカル
		{"ff02::1", true}, // IPv6 リンクローカルマルチキャスト

		{"93.184.216.34", false},
		{"8.8.8.8", false},
		{"2606:2800:220:1:248:1893:25c8:1946", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("テストデータが不正: %s", tt.ip)
			}
			if got := IsInternalIP(ip); got != tt.want {
				t.Errorf("IsInternalIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
