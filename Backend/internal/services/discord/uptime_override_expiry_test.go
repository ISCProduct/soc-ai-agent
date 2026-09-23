package discord

import (
	"context"
	"strings"
	"testing"
)

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

// TestProdCommand_UntilOption は /prod の state と until が
// **実ハンドラ経由で** SSM に保存されることを固定する。
//
// 以前はここでハンドラを呼ばず、結合処理をテスト内に複製して ParseOverride だけを
// 検証していた。そのため handler.go の until 取得・結合を削除しても全ケースが通り、
// この機能の退行をまったく検出できなかった。AGENTS.md の
// 「実ハンドラへ渡して、SSMに保存される値と応答を検証する」に従う。
func TestProdCommand_UntilOption(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		until       string
		wantStored  string
		wantInReply string
		wantNoWrite bool
	}{
		{
			name: "until 省略なら従来どおり", state: "off", until: "",
			wantStored: "off", wantInReply: "常時停止",
		},
		{
			name: "until 指定で復帰日つきが保存される", state: "off", until: "2026-09-24",
			wantStored: "off:2026-09-24", wantInReply: "2026-09-24",
		},
		{
			name: "on にも付けられる", state: "on", until: "2026-10-01",
			wantStored: "on:2026-10-01", wantInReply: "2026-10-01",
		},
		{
			name: "until の前後空白は無視", state: "off", until: "  2026-09-24  ",
			wantStored: "off:2026-09-24", wantInReply: "2026-09-24",
		},
		{
			name: "auto に until は付けられない", state: "auto", until: "2026-09-24",
			wantNoWrite: true,
		},
		{
			name: "不正な日付は書き込みへ進まない", state: "off", until: "9/24",
			wantNoWrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssm := &fakeSSM{value: "auto"}
			h := NewHandler(newTestUptimeService(ssm), &stagingSetterStub{}, noopDispatcher(), "", "role-allowed")

			options := []CommandOption{{Name: OptionNameState, Value: tt.state}}
			if tt.until != "" {
				options = append(options, CommandOption{Name: OptionNameUntil, Value: tt.until})
			}
			resp := h.handleCommand(context.Background(), &Interaction{
				Type:   InteractionTypeApplicationCommand,
				Member: &Member{Roles: []string{"role-allowed"}},
				Data:   &InteractionData{Name: CommandNameProd, Options: options},
			})

			if tt.wantNoWrite {
				if ssm.putCalls > 0 {
					t.Errorf("不正な指定なのにSSMへ書き込んでいる: %q", ssm.putValue)
				}
				return
			}
			if ssm.putCalls != 1 {
				t.Fatalf("SSMへの書き込み回数 = %d, want 1", ssm.putCalls)
			}
			if ssm.putValue != tt.wantStored {
				t.Errorf("保存値 = %q, want %q", ssm.putValue, tt.wantStored)
			}
			// 応答が実際の設定内容と食い違うと、停止固定にしたのに
			// 「autoへ戻しました」と読めてしまう。
			if got := contentOf(resp); !strings.Contains(got, tt.wantInReply) {
				t.Errorf("応答に %q が含まれない: %q", tt.wantInReply, got)
			}
		})
	}
}

// TestOverrideAppliedMessage_Expiry は確認メッセージが復帰日つきでも
// 実際の設定内容を伝えることを固定する。
func TestOverrideAppliedMessage_Expiry(t *testing.T) {
	tests := []struct {
		state      string
		wantHas    []string
		wantNotHas []string
	}{
		{state: "off", wantHas: []string{"常時停止"}, wantNotHas: []string{"自動復帰"}},
		{state: "on", wantHas: []string{"常時起動"}, wantNotHas: []string{"自動復帰"}},
		{state: "auto", wantHas: []string{"日付リストに従う"}},
		{
			state:      "off:2026-09-24",
			wantHas:    []string{"常時停止", "2026-09-24", "自動復帰"},
			wantNotHas: []string{"戻しました"},
		},
		{
			state:      "on:2026-10-01",
			wantHas:    []string{"常時起動", "2026-10-01", "自動復帰"},
			wantNotHas: []string{"戻しました"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := overrideAppliedMessage(tt.state)
			for _, w := range tt.wantHas {
				if !strings.Contains(got, w) {
					t.Errorf("%q が含まれない: %q", w, got)
				}
			}
			for _, w := range tt.wantNotHas {
				if strings.Contains(got, w) {
					t.Errorf("%q が含まれてはいけない: %q", w, got)
				}
			}
		})
	}
}
