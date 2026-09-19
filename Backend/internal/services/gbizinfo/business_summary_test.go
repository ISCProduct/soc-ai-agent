package gbizinfo

// 事業概要の整形テスト。
// 実行: cd Backend && go test ./internal/services/gbizinfo/ -run TestFlattenBusinessSummary -v

import "testing"

func TestFlattenBusinessSummary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			// gBizINFO は複数事業を改行で区切って返す（サイバーエージェントの実データ）。
			// そのまま main_business に入れると1行表示の画面で崩れる。
			name: "改行区切りを読点でつなぐ",
			in:   "メディア事業\nインターネット広告事業\nゲーム事業\n投資育成事業",
			want: "メディア事業、インターネット広告事業、ゲーム事業、投資育成事業",
		},
		{
			name: "1行ならそのまま",
			in:   "自動車事業および金融その他の事業",
			want: "自動車事業および金融その他の事業",
		},
		{
			name: "CRLFも区切りとして扱う",
			in:   "事業A\r\n事業B",
			want: "事業A、事業B",
		},
		{
			name: "空行は落とす",
			in:   "事業A\n\n\n事業B\n",
			want: "事業A、事業B",
		},
		{
			name: "前後の空白を落とす",
			in:   "  事業A  \n  事業B  ",
			want: "事業A、事業B",
		},
		{
			name: "空文字",
			in:   "",
			want: "",
		},
		{
			name: "空白だけ",
			in:   "  \n  ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flattenBusinessSummary(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
