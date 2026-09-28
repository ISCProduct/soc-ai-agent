package textsim

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// 小数点も約物として除去されるが、比較の両側で同じように落ちるため一致判定には影響しない
		{name: "全角英数字はNFKCで半角化し小文字にする", in: "Ｗｅｂサイト１．５倍", want: "webサイト15倍"},
		{name: "空白と改行は除去する", in: " 貴社の\nサービス　です ", want: "貴社のサービスです"},
		{name: "約物は除去する", in: "資格は簿記2級です、実務で活用（予定）。", want: "資格は簿記2級です実務で活用予定"},
		{name: "長音記号は残す", in: "サーバー運用", want: "サーバー運用"},
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
		name    string
		a       string
		b       string
		wantMin float64
		wantMax float64
	}{
		{name: "完全一致は1.0", a: "接客で売上に貢献しました。", b: "接客で売上に貢献しました。", wantMin: 1, wantMax: 1},
		{name: "部分一致は1.0", a: "アルバイトでは接客で売上に貢献しました。", b: "接客で売上に貢献", wantMin: 1, wantMax: 1},
		{name: "語尾違いは高スコア", a: "接客で売上に貢献しました。", b: "接客で売上に貢献した", wantMin: 0.7, wantMax: 0.999},
		{name: "無関係は低スコア", a: "普通自動車免許を取得しています。", b: "接客で売上に貢献した", wantMin: 0, wantMax: 0.3},
		{name: "空文字は0", a: "", b: "接客で売上に貢献した", wantMin: 0, wantMax: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Similarity(tt.a, tt.b)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("Similarity() = %.3f, want %.3f〜%.3f", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
