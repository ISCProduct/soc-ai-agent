package company

// 公式サイトからの本文収集のテスト。
// 実行: cd Backend && go test ./internal/services/company/ -run TestWebsite -v

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// filler は本文の文字数閾値を満たすためのダミー本文を作る。
func filler(n int) string {
	return strings.Repeat("当社は東京都港区に本社を置く企業です。", n)
}

func TestWebsite_会社概要ページを辿って本文を集める(t *testing.T) {
	var gotPaths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		switch r.URL.Path {
		case "/":
			fmt.Fprintf(w, `<html><body>
				<a href="/news/">ニュース</a>
				<a href="/company/books/">書籍紹介</a>
				<a href="/company/">会社概要</a>
				<p>トップページ本文</p>
			</body></html>`)
		case "/company/":
			fmt.Fprintf(w, `<html><body><h1>会社概要</h1><p>設立 2010年 従業員数 500名 資本金 1億円</p><p>%s</p></body></html>`, filler(60))
		default:
			fmt.Fprintf(w, `<html><body><p>%s</p></body></html>`, filler(60))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	text, err := collectWebsiteText(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("本文が取れなかった: %v", err)
	}

	if !strings.Contains(text, "トップページ本文") {
		t.Error("トップページの本文が含まれていない")
	}
	if !strings.Contains(text, "設立 2010年") {
		t.Error("会社概要ページの本文が含まれていない")
	}

	// ラベルが「会社概要」の /company/ を、紛らわしい /company/books/ より先に読む
	if len(gotPaths) < 2 || gotPaths[1] != "/company/" {
		t.Errorf("会社概要ページを最優先で読むべき: %v", gotPaths)
	}
}

func TestWebsite_本文が薄ければエラーにする(t *testing.T) {
	// JavaScriptで描画するサイトはHTMLにほとんど文字が無い。
	// ここで通してしまうと、中身の無いテキストをLLMに投げて課金だけ発生する。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><div id="root"></div><script>render()</script></body></html>`)
	}))
	defer srv.Close()

	if _, err := collectWebsiteText(context.Background(), srv.URL); err == nil {
		t.Error("本文が薄いのでエラーを返すべき")
	}
}

func TestWebsite_URLが空ならエラー(t *testing.T) {
	if _, err := collectWebsiteText(context.Background(), "   "); err == nil {
		t.Error("空URLはエラーにすべき")
	}
}

func TestWebsite_上限で切り詰める(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body><p>%s</p></body></html>`, filler(4000))
	}))
	defer srv.Close()

	text, err := collectWebsiteText(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("エラーが返った: %v", err)
	}
	if got := len([]rune(text)); got > maxWebsiteTextRunes {
		t.Errorf("上限 %d を超えている: %d", maxWebsiteTextRunes, got)
	}
}

func TestRankAboutLinks(t *testing.T) {
	const base = "https://example.co.jp/"

	tests := []struct {
		name string
		html string
		want string // 先頭に来てほしいURL。空なら候補ゼロを期待
	}{
		{
			name: "ラベル完全一致を最優先する",
			html: `<a href="/corporate/ir/">IR情報</a><a href="/about/">会社概要</a>`,
			want: "https://example.co.jp/about/",
		},
		{
			name: "同じラベルなら浅い階層を優先する",
			html: `<a href="/company/profile/detail/">会社概要</a><a href="/company/">会社概要</a>`,
			want: "https://example.co.jp/company/",
		},
		{
			name: "ラベルが無くてもURLの手がかりで拾う",
			html: `<a href="/corporate/"><img src="logo.png"></a>`,
			want: "https://example.co.jp/corporate/",
		},
		{
			name: "外部サイトへは出ていかない",
			html: `<a href="https://recruit.example.com/company/">会社概要</a>`,
			want: "",
		},
		{
			name: "無関係なリンクは候補にしない",
			html: `<a href="/news/">ニュース</a><a href="/contact/">お問い合わせ</a>`,
			want: "",
		},
		{
			name: "同じURLは重複させない",
			html: `<a href="/company/">会社概要</a><a href="/company/">企業情報</a>`,
			want: "https://example.co.jp/company/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rankAboutLinks(tt.html, base)

			if tt.want == "" {
				if len(got) != 0 {
					t.Errorf("候補ゼロのはずが %v", got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("候補が空。%s を期待", tt.want)
			}
			if got[0] != tt.want {
				t.Errorf("先頭が違う: got %s, want %s (全体=%v)", got[0], tt.want, got)
			}
			if tt.name == "同じURLは重複させない" && len(got) != 1 {
				t.Errorf("重複が除去されていない: %v", got)
			}
		})
	}
}
