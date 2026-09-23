package discord

import "testing"

// TestParseOverride_Expiry は復帰日付きオーバーライドの受理と拒否を固定する。
//
// 期限の無い off は戻し忘れると起動日が丸ごと潰れる。実際 2026-09-12 に設定された
// off が放置され、9/24 の起動日を潰しかけたうえ、本番デプロイのマイグレーションが
// 3回失敗していた（connection timed out / no route to host）。
func TestParseOverride_Expiry(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "従来の on", raw: "on", want: "on"},
		{name: "従来の off", raw: "off", want: "off"},
		{name: "従来の auto", raw: "auto", want: "auto"},
		{name: "空文字は auto", raw: "", want: "auto"},
		{name: "大文字と空白は無視", raw: "  OFF  ", want: "off"},

		{name: "復帰日つき off", raw: "off:2026-09-24", want: "off:2026-09-24"},
		{name: "復帰日つき on", raw: "on:2026-10-01", want: "on:2026-10-01"},
		{name: "復帰日つきも大文字を無視", raw: "OFF:2026-09-24", want: "off:2026-09-24"},

		{name: "auto に復帰日は付けられない", raw: "auto:2026-09-24", wantErr: true},
		{name: "日付の書式違い", raw: "off:2026/09/24", wantErr: true},
		{name: "日付が短い", raw: "off:2026-9-24", wantErr: true},
		{name: "実在しない日付", raw: "off:2026-02-30", wantErr: true},
		{name: "日付が空", raw: "off:", wantErr: true},
		{name: "未知の状態", raw: "pause", wantErr: true},
		{name: "未知の状態に日付", raw: "pause:2026-09-24", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOverride(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseOverride(%q) はエラーになるべき（got %q）", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOverride(%q) = error %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("ParseOverride(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestResolveOverride は「復帰日に達したら auto へ戻る」ことを固定する。
// この判定は prod-uptime-scheduler.yml 側にも同じものがあり、両方が一致している必要がある。
func TestResolveOverride(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		today string
		want  string
	}{
		{name: "期限なし off はいつでも off", raw: "off", today: "2026-12-31", want: "off"},
		{name: "期限なし on はいつでも on", raw: "on", today: "2026-12-31", want: "on"},
		{name: "auto はそのまま", raw: "auto", today: "2026-09-23", want: "auto"},

		{name: "復帰日の前日はまだ off", raw: "off:2026-09-24", today: "2026-09-23", want: "off"},
		{name: "復帰日当日は auto へ戻る", raw: "off:2026-09-24", today: "2026-09-24", want: "auto"},
		{name: "復帰日を過ぎていれば auto", raw: "off:2026-09-24", today: "2026-09-25", want: "auto"},
		{name: "on も同じく復帰する", raw: "on:2026-09-24", today: "2026-09-24", want: "auto"},

		{name: "年跨ぎも文字列比較で正しい", raw: "off:2027-01-01", today: "2026-12-31", want: "off"},
		{name: "年跨ぎ当日", raw: "off:2027-01-01", today: "2027-01-01", want: "auto"},

		// 壊れた値で本番を起動しっぱなしにする/落とすより、日付リストに従う方が安全。
		{name: "壊れた日付は auto に倒す", raw: "off:garbage", today: "2026-09-23", want: "auto"},
		{name: "未知の状態は auto に倒す", raw: "pause", today: "2026-09-23", want: "auto"},
		{name: "空文字は auto", raw: "", today: "2026-09-23", want: "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveOverride(tt.raw, tt.today); got != tt.want {
				t.Errorf("ResolveOverride(%q, %q) = %q, want %q", tt.raw, tt.today, got, tt.want)
			}
		})
	}
}

// TestOverrideLabel_ShowsExpiry は状態表示に復帰日が出ることを固定する。
// 期限が見えないと、表示を見ても戻し忘れかどうか判断できない。
func TestOverrideLabel_ShowsExpiry(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{state: "on", want: "常時起動"},
		{state: "off", want: "常時停止"},
		{state: "auto", want: "日付リストに従う"},
		{state: "off:2026-09-24", want: "常時停止（2026-09-24 に auto へ復帰）"},
		{state: "on:2026-10-01", want: "常時起動（2026-10-01 に auto へ復帰）"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := overrideLabel(tt.state); got != tt.want {
				t.Errorf("overrideLabel(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

// TestSetProdOverride_UntilOption は /prod の state と until が結合されることを固定する。
//
// state は Discord 側で選択肢(on/off/auto)固定になっており日付を入力できない。
// until を別オプションで受けて結合しないと、復帰日つきオーバーライドを
// Discord から設定できず、機能が存在しないのと同じになる。
func TestSetProdOverride_UntilOption(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		until   string
		want    string
		wantErr bool
	}{
		{name: "until 省略なら従来どおり", state: "off", until: "", want: "off"},
		{name: "until 指定で結合される", state: "off", until: "2026-09-24", want: "off:2026-09-24"},
		{name: "on にも付けられる", state: "on", until: "2026-10-01", want: "on:2026-10-01"},
		{name: "until の前後空白は無視", state: "off", until: "  2026-09-24  ", want: "off:2026-09-24"},
		{name: "auto に until は付けられない", state: "auto", until: "2026-09-24", wantErr: true},
		{name: "不正な日付は弾く", state: "off", until: "9/24", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// handler と同じ結合手順を通す
			raw := tt.state
			if u := trimSpaceForTest(tt.until); u != "" {
				raw = trimSpaceForTest(raw) + ":" + u
			}
			got, err := ParseOverride(raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("state=%q until=%q はエラーになるべき（got %q）", tt.state, tt.until, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("state=%q until=%q: %v", tt.state, tt.until, err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func trimSpaceForTest(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
