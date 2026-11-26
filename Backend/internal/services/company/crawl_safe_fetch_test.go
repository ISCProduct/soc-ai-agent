package company

// 外部サイト取得が内部アドレスへ行かないことを確かめる(#2489)。
// 実在のサイトには接続しない。宛先はすべて httptest か、接続前に拒否される URL。
// 実行: cd Backend && go test ./internal/services/company/ -run TestSafeFetch -v

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"Backend/internal/netsafe"
)

// allowTestServer は httptest のサーバーだけを取得の検査から外す（テスト専用）。
func allowTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	prev := testAllowedAddrs
	testAllowedAddrs = append(append([]string(nil), prev...), u.Host)
	t.Cleanup(func() { testAllowedAddrs = prev })
}

// countingServer は届いたリクエスト数を数えるサーバー。内部の資源に見立てる。
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("secret"))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// 取得の入口ごとに、同じ検査を通っているかを見る。
var safeFetchEntries = []struct {
	name  string
	fetch func(rawURL string) error
}{
	{"fetchBytes", func(u string) error { _, _, err := fetchBytes(u); return err }},
	{"fetchText", func(u string) error { _, err := fetchText(u); return err }},
	{"fetchOpenworkBytes", func(u string) error { _, _, err := fetchOpenworkBytes(u); return err }},
	{"fetchRobots", func(u string) error { _, err := fetchRobots(context.Background(), u); return err }},
	{"fetchPageHTML", func(u string) error {
		_, err := fetchPageHTML(context.Background(), parseRobots(""), u)
		return err
	}},
}

func TestSafeFetch_内部宛は接続せず拒否する(t *testing.T) {
	internal, hits := countingServer(t)
	port := internal.URL[strings.LastIndex(internal.URL, ":")+1:]

	// 公開ホスト名に見えるが、名前解決するとループバックを返す（DNSの差し替え）。
	origLookup := netsafe.LookupIP
	netsafe.LookupIP = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil }
	t.Cleanup(func() { netsafe.LookupIP = origLookup })

	// wantErr は拒否したのがどちらの検査かを表す。
	// URL の表記で分かるものは取得前の検査で、名前解決して初めて分かるものは接続直前(netsafe)で止まる。
	const byURLCheck, byDial = "not allowed", "blocked request to internal address"
	targets := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"ループバックのIP", internal.URL + "/robots.txt", byURLCheck},
		{"localhost", "http://localhost:" + port + "/robots.txt", byURLCheck},
		{"プライベートIP", "http://10.0.0.1/robots.txt", byURLCheck},
		{"メタデータのリンクローカル", "http://169.254.169.254/latest/meta-data/", byURLCheck},
		{"内部へ解決されるホスト名", "http://rebind.test:" + port + "/robots.txt", byDial},
		{"http/https 以外", "ftp://example.com/robots.txt", "only http/https"},
	}
	for _, e := range safeFetchEntries {
		for _, tt := range targets {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				before := hits.Load()
				err := e.fetch(tt.url)
				if err == nil {
					t.Fatalf("拒否されるべき: %s", tt.url)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("想定の検査で止まっていない（want %q）: %v", tt.wantErr, err)
				}
				if got := hits.Load() - before; got != 0 {
					t.Errorf("内部のサーバーへ %d 回届いた", got)
				}
			})
		}
	}
}

func TestSafeFetch_内部へのリダイレクトはリダイレクトの段で止める(t *testing.T) {
	internal, hits := countingServer(t)

	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL+r.URL.Path, http.StatusFound)
	}))
	defer public.Close()
	allowTestServer(t, public)

	for _, e := range safeFetchEntries {
		t.Run(e.name, func(t *testing.T) {
			before := hits.Load()
			err := e.fetch(public.URL + "/robots.txt")
			if err == nil {
				t.Fatal("内部へのリダイレクトは拒否されるべき")
			}
			// 接続より前（CheckRedirect の URL 検査）で止まっていること。
			if !strings.Contains(err.Error(), "not allowed") {
				t.Errorf("リダイレクトの検査で止まっていない: %v", err)
			}
			if got := hits.Load() - before; got != 0 {
				t.Errorf("内部のサーバーへ %d 回届いた", got)
			}
		})
	}
}

func TestSafeFetch_許可した公開先は従来どおり取得できる(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("User-agent: *\nDisallow:\n"))
	}))
	defer srv.Close()
	allowTestServer(t, srv)

	for _, e := range safeFetchEntries {
		t.Run(e.name, func(t *testing.T) {
			if err := e.fetch(srv.URL + "/robots.txt"); err != nil {
				t.Fatalf("取得できるべき: %v", err)
			}
		})
	}
}

func TestSafeFetch_InvalidURLErrorHasNoSecret(t *testing.T) {
	err := checkFetchURL("https://example.com:bad/path?token=SECRET")
	if err == nil {
		t.Fatal("不正な URL が通った")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("エラーに元の URL が入っている: %v", err)
	}
}

func TestSafeFetch_TransportHasIdleLimits(t *testing.T) {
	if fetchTransport.MaxIdleConns == 0 || fetchTransport.IdleConnTimeout == 0 {
		t.Fatalf("アイドル接続の上限・期限が無い: max=%d timeout=%v", fetchTransport.MaxIdleConns, fetchTransport.IdleConnTimeout)
	}
}
