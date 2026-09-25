// Package netsafe は外部URLを読む処理で共通に使う SSRF 対策を持つ。
//
// 以前は resume と companyfetch がそれぞれ別の実装を持っており、片方だけが
// 安全だった（#1408）。
//
//	resume       : 接続時に解決済みIPを検査する（正しい）
//	companyfetch : ホスト名を解決して検査 → その後 http.Client が改めて解決して接続
//
// 後者は検査と接続で別々に名前解決するため、短いTTLを返すDNSサーバーを使えば
// 検査後に内部アドレスへ差し替えられる（DNSリバインディング）。リダイレクト先も
// 同じ関数で検査していたので同様に通る。
//
// ここに1つだけ置き、両方から使う。
package netsafe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// LookupIP はホスト名からIPを解決する。テストで差し替えるために変数にしている。
var LookupIP = net.LookupIP

// IsInternalIP は外部へ出してはいけないアドレスかを判定する。
//
// リンクローカル（169.254.0.0/16）を含めるのは、クラウドのメタデータ
// エンドポイント 169.254.169.254 を塞ぐため。ここを抜かれると
// インスタンスの資格情報が読まれる。
func IsInternalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}

// DialContext は接続直前に実際に使うIPを検査し、そのIPへ直接つなぐ。
//
// 解決結果をそのまま宛先に使うのが要点。標準ダイヤラにホスト名を渡すと
// もう一度名前解決が走り、検査したIPと接続先がずれる余地が残る。
func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	if ip := net.ParseIP(host); ip != nil {
		if IsInternalIP(ip) {
			return nil, fmt.Errorf("blocked request to internal address: %s", host)
		}
		return dialer.DialContext(ctx, network, addr)
	}

	ips, err := LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("failed to resolve host: %s", host)
	}
	// 1つでも内部アドレスが混ざっていたら接続しない。
	// 「公開IPと内部IPの両方を返す」応答で内部側を引かせる手を塞ぐ。
	for _, ip := range ips {
		if IsInternalIP(ip) {
			return nil, fmt.Errorf("blocked request to internal address: %s", host)
		}
	}

	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to resolve host: %s", host)
	}
	return nil, lastErr
}

// NewTransport は DialContext を組み込んだ http.Transport を返す。
func NewTransport() *http.Transport {
	return &http.Transport{DialContext: DialContext}
}
