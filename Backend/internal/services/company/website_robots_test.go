package company

// robots.txt の解釈と、公式サイト取得での尊重を確かめる。
// 実行: cd Backend && go test ./internal/services/company/ -run 'TestRobots|TestWebsite' -v

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRobotsAllows(t *testing.T) {
	tests := []struct {
		name   string
		robots string
		path   string
		want   bool
	}{
		{"空なら許可", "", "/company/", true},
		{"全体禁止", "User-agent: *\nDisallow: /", "/company/", false},
		{"空のDisallowは制限なし", "User-agent: *\nDisallow:", "/company/", true},
		{"前方一致で禁止", "User-agent: *\nDisallow: /company", "/company/profile", false},
		{"対象外のパスは許可", "User-agent: *\nDisallow: /private/", "/company/", true},
		{"長い方の規則が勝つ(Allow)", "User-agent: *\nDisallow: /company/\nAllow: /company/profile", "/company/profile", true},
		{"長い方の規則が勝つ(Disallow)", "User-agent: *\nAllow: /company/\nDisallow: /company/secret", "/company/secret/x", false},
		{"同じ長さならAllow", "User-agent: *\nDisallow: /a\nAllow: /a", "/a", true},
		{"ワイルドカード", "User-agent: *\nDisallow: /*.pdf$", "/ir/report.pdf", false},
		{"$は末尾一致のみ", "User-agent: *\nDisallow: /*.pdf$", "/ir/report.pdf.html", true},
		{"クエリも照合対象", "User-agent: *\nDisallow: /*?print=", "/company/?print=1", false},
		{"コメントと大文字小文字を無視", "# x\nUSER-AGENT: *  # all\nDISALLOW: /company # c", "/company/", false},
		// 自分宛のグループがあれば * より優先し、* の規則は使わない
		{"自分宛のグループを優先", "User-agent: *\nDisallow: /\n\nUser-agent: SocAI\nDisallow: /private/", "/company/", true},
		{"自分宛のグループで禁止", "User-agent: *\nAllow: /\n\nUser-agent: socai\nDisallow: /company/", "/company/", false},
		{"他のボット宛は無関係", "User-agent: Googlebot\nDisallow: /", "/company/", true},
		{"複数UA行は同じグループ", "User-agent: Googlebot\nUser-agent: SocAI\nDisallow: /company/", "/company/", false},
		{"robots.txt自体は常に許可", "User-agent: *\nDisallow: /", "/robots.txt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parseRobots(tt.robots)
			if got := r.allows("https://example.co.jp" + tt.path); got != tt.want {
				t.Errorf("allows(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFetchRobots(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		allowed bool // /company/ を読めるか
	}{
		{"200なら中身に従う", http.StatusOK, "User-agent: *\nDisallow: /company/", false, false},
		// RFC 9309: 4xx は robots.txt が無いものとして扱い、制限なし
		{"404なら制限なし", http.StatusNotFound, "", false, true},
		// RFC 9309: 5xx は全面禁止とみなす。取得自体をやめる
		{"500なら取得しない", http.StatusInternalServerError, "", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUA string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUA = r.UserAgent()
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()
			t.Setenv("COMPANY_BOT_CONTACT", "https://bot.example.test/")

			r, err := fetchRobots(context.Background(), srv.URL+"/")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && r.allows(srv.URL+"/company/") != tt.allowed {
				t.Errorf("allows(/company/) = %v, want %v", !tt.allowed, tt.allowed)
			}
			if !strings.Contains(gotUA, "https://bot.example.test/") {
				t.Errorf("User-Agent に連絡先が無い: %q", gotUA)
			}
		})
	}
}

func TestWebsite_robotsで禁止されたページは読まない(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprint(w, "User-agent: *\nDisallow: /company/")
		case "/":
			fmt.Fprintf(w, `<html><body><a href="/company/">会社概要</a><a href="/about/">企業情報</a><p>%s</p></body></html>`, filler(60))
		default:
			fmt.Fprintf(w, `<html><body><p>%s</p></body></html>`, filler(60))
		}
	}))
	defer srv.Close()

	if _, err := collectWebsiteText(context.Background(), srv.URL+"/"); err != nil {
		t.Fatalf("本文が取れなかった: %v", err)
	}
	for _, p := range gotPaths {
		if strings.HasPrefix(p, "/company/") {
			t.Errorf("robots.txt で禁止された %s を取得した: %v", p, gotPaths)
		}
	}
	if !strings.Contains(strings.Join(gotPaths, " "), "/about/") {
		t.Errorf("許可されたページは読むべき: %v", gotPaths)
	}
}

func TestWebsite_トップが禁止なら何も読まない(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		if r.URL.Path == "/robots.txt" {
			fmt.Fprint(w, "User-agent: *\nDisallow: /")
			return
		}
		fmt.Fprintf(w, `<html><body><p>%s</p></body></html>`, filler(60))
	}))
	defer srv.Close()

	if _, err := collectWebsiteText(context.Background(), srv.URL+"/"); err == nil {
		t.Error("全面禁止なのでエラーを返すべき")
	}
	if len(gotPaths) != 1 {
		t.Errorf("robots.txt 以外を取得した: %v", gotPaths)
	}
}

func TestWebsite_robotsが5xxなら何も読まない(t *testing.T) {
	var gotPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `<html><body><p>%s</p></body></html>`, filler(60))
	}))
	defer srv.Close()

	if _, err := collectWebsiteText(context.Background(), srv.URL+"/"); err == nil {
		t.Error("robots.txt を確認できないのでエラーを返すべき")
	}
	if len(gotPaths) != 1 {
		t.Errorf("robots.txt 以外を取得した: %v", gotPaths)
	}
}
