package company

// normalizeHTMLText のテスト。
// 実行: cd Backend && go test ./internal/services/company/ -run TestNormalizeHTMLText -v

import "testing"

func TestNormalizeHTMLText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// Goの正規表現(RE2)は後方参照を持たない。`</\1>` を書くと
			// MustCompile が panic し、この関数を呼ぶ経路が丸ごと落ちていた。
			name: "scriptとstyleの中身を落とす",
			in:   `<html><head><style>body{color:red}</style></head><body><script>var a=1;</script><p>本文</p></body></html>`,
			want: "本文",
		},
		{
			name: "閉じタグの種類が入れ替わっていても取り違えない",
			in:   `<script>a</script><p>あ</p><style>b</style><p>い</p>`,
			want: "あ い",
		},
		{
			name: "HTMLエンティティを戻す",
			in:   `<p>A&amp;B &lt;株式会社&gt;</p>`,
			want: "A&B <株式会社>",
		},
		{
			name: "ノーブレークスペースと連続空白を1つにする",
			in:   "<p>東京都  港区   六本木</p>",
			want: "東京都 港区 六本木",
		},
		{
			name: "属性に > を含まないタグは除去する",
			in:   `<a href="/company" class="nav">会社概要</a>`,
			want: "会社概要",
		},
		{
			name: "空文字",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 修正前はここで panic していた。
			got := normalizeHTMLText(tt.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeHTMLText_呼び出し元が落ちないこと は、実際の呼び出し経路と同じ
// 大きめのHTMLでも panic しないことを確認する。
func TestNormalizeHTMLText_大きなHTMLでも落ちない(t *testing.T) {
	var sb []byte
	for range 200 {
		sb = append(sb, []byte(`<div class="item"><script>x</script><span>求人</span></div>`)...)
	}
	if got := normalizeHTMLText(string(sb)); got == "" {
		t.Error("本文が空になった")
	}
}
