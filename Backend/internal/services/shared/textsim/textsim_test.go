package textsim

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "全角英数字はNFKCで半角化し小文字にする", in: "Ｗｅｂサイト", want: "webサイト"},
		{name: "空白と改行は除去する", in: " 貴社の\nサービス　です ", want: "貴社のサービスです"},
		{name: "約物は除去する", in: "資格は簿記2級です、実務で活用（予定）。", want: "資格は簿記2級です実務で活用予定"},
		{name: "長音記号は残す", in: "サーバー運用", want: "サーバー運用"},
		{name: "数字に挟まれた小数点・桁区切りは残す", in: "売上1.5倍／1,200万円", want: "売上1.5倍1,200万円"},
		{name: "数字に挟まれない記号は除去する", in: "以上.。", want: "以上"},
		{name: "波ダッシュは除去する", in: "2020〜2023年", want: "20202023年"},
		{name: "全角チルダも同じ結果になる", in: "2020～2023年", want: "20202023年"},
		{name: "空文字", in: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Normalize(tt.in); got != tt.want {
				t.Errorf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		name     string
		haystack string
		needle   string
		wantMin  float64
		wantMax  float64
	}{
		{
			name: "完全一致は1.0", haystack: "接客で売上に貢献しました。", needle: "接客で売上に貢献しました。",
			wantMin: 1, wantMax: 1,
		},
		{
			name: "本文が引用を含めば1.0", haystack: "アルバイトでは接客で売上に貢献しました。", needle: "接客で売上に貢献",
			wantMin: 1, wantMax: 1,
		},
		{
			// 逆向きを1.0にすると「年」「なし」のような短い表ヘッダが常に勝ってしまう
			name: "引用が本文を含む場合は1.0にしない", haystack: "年", needle: "平成30年4月に入学しました",
			wantMin: 0, wantMax: 0.2,
		},
		{
			name:     "引用が本文を含む場合は重なりの長さに応じたスコアになる",
			haystack: "学生時代はサークルの新人勧誘を担当し、", needle: "学生時代はサークルの新人勧誘を担当し、50名の入会につなげました。",
			wantMin: 0.7, wantMax: 0.75,
		},
		{
			name: "語尾違いは高スコア", haystack: "接客で売上に貢献しました。", needle: "接客で売上に貢献した",
			wantMin: 0.7, wantMax: 0.999,
		},
		{
			name: "数値違いは完全一致にならない", haystack: "売上を1.5倍にしました", needle: "売上を15倍にしました",
			wantMin: 0.8, wantMax: 0.999,
		},
		{
			name: "無関係は低スコア", haystack: "普通自動車免許を取得しています。", needle: "接客で売上に貢献した",
			wantMin: 0, wantMax: 0.3,
		},
		{name: "空文字は0", haystack: "", needle: "接客で売上に貢献した", wantMin: 0, wantMax: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.haystack, tt.needle)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Score() = %.3f, want %.3f〜%.3f", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBigramsLen(t *testing.T) {
	// Len は正規化後の文字数（約物・空白を除いた数）
	if got := New(" 売上を、1.5倍に　しました。").Len(); got != 12 {
		t.Errorf("Len() = %d, want 12 (%q)", got, New(" 売上を、1.5倍に　しました。").Norm())
	}
}
