package companyfetch

import (
	"context"
	"net"
	"strings"
	"testing"

	"Backend/internal/netsafe"
)

// TestFetchURLText_BlocksDNSRebinding は FetchURLText が接続直前のIP検査で
// 止まることを固定する（#1408）。
//
// ValidatePublicHTTPURL はホスト名を見るだけなので、公開ホスト名が内部IPへ
// 解決されるケースを通してしまう。検査と接続で別々に名前解決していたため、
// 短いTTLを返すDNSサーバーで検査後に差し替えられた（DNSリバインディング）。
//
// ホスト名の検査だけが直っても意味が無いので、実際に FetchURLText を呼んで
// 接続に至らないことを見る。
func TestFetchURLText_BlocksDNSRebinding(t *testing.T) {
	original := netsafe.LookupIP
	defer func() { netsafe.LookupIP = original }()

	tests := []struct {
		name string
		ips  []net.IP
	}{
		{name: "メタデータエンドポイントへ解決される", ips: []net.IP{net.ParseIP("169.254.169.254")}},
		{name: "VPC内部へ解決される", ips: []net.IP{net.ParseIP("10.20.1.85")}},
		{name: "ループバックへ解決される", ips: []net.IP{net.ParseIP("127.0.0.1")}},
		{name: "公開IPと内部IPが混在", ips: []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.1")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netsafe.LookupIP = func(string) ([]net.IP, error) { return tt.ips, nil }

			// ホスト名としては公開サイトに見えるため ValidatePublicHTTPURL は通る。
			if err := ValidatePublicHTTPURL("https://looks-public.example.com/careers"); err != nil {
				t.Fatalf("前提が崩れている: ホスト名検査で弾かれた: %v", err)
			}

			_, err := FetchURLText(context.Background(), "https://looks-public.example.com/careers")
			if err == nil {
				t.Fatal("内部アドレスへ解決されるホストの取得はエラーになるべき")
			}
			if !strings.Contains(err.Error(), "internal address") {
				t.Errorf("接続時のIP検査で止まっていない: %v", err)
			}
		})
	}
}

// TestFetchURLText_RejectsInternalHostname は従来のホスト名検査も残っていることを見る。
// Transport 側へ寄せた結果こちらが外れると、名前解決前に弾ける分の防御が減る。
func TestFetchURLText_RejectsInternalHostname(t *testing.T) {
	for _, raw := range []string{
		"http://localhost/x",
		"http://127.0.0.1/x",
		"http://10.0.0.1/x",
		"ftp://example.com/x",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := ValidatePublicHTTPURL(raw); err == nil {
				t.Errorf("%s は弾かれるべき", raw)
			}
		})
	}
}
