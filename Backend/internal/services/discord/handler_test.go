package discord

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type stagingSetterStub struct {
	mu     sync.Mutex
	values []string
	err    error
}

func (s *stagingSetterStub) SetState(_ context.Context, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.values = append(s.values, v)
	return nil
}

func (s *stagingSetterStub) called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.values)
}

// commandInteraction は /prod や /staging の呼び出しペイロードを作る。
func commandInteraction(name string, member *Member, state string) *Interaction {
	return &Interaction{
		Type:   InteractionTypeApplicationCommand,
		Member: member,
		Data: &InteractionData{
			Name:    name,
			Options: []CommandOption{{Name: OptionNameState, Value: state}},
		},
	}
}

func contentOf(resp *InteractionResponse) string {
	if resp == nil || resp.Data == nil {
		return ""
	}
	return resp.Data.Content
}

// 未設定のディスパッチャ。トークンが無いので何も起動せず (false, nil) を返す。
func noopDispatcher() *WorkflowDispatcher { return &WorkflowDispatcher{} }

// /prod と /staging は本番や開発チーム全体の環境を止められるので、
// ロール制限が唯一の防御になる。ルーティング側から権限チェックを呼び忘れても
// hasAllowedRole の単体テストでは検出できないため、コマンド経由で検証する。
func TestHandler_Authorization(t *testing.T) {
	members := []struct {
		name       string
		member     *Member
		wantDenied bool
	}{
		{"許可ロールを持つ", &Member{Roles: []string{"role-allowed"}}, false},
		{"別のロールしか持たない", &Member{Roles: []string{"role-other"}}, true},
		{"ロールなし", &Member{Roles: []string{}}, true},
		{"DMなど Member が無い", nil, true},
	}

	for _, command := range []string{CommandNameProd, CommandNameStaging, CommandNameProdUptime} {
		for _, tt := range members {
			t.Run(command+"/"+tt.name, func(t *testing.T) {
				staging := &stagingSetterStub{}
				h := NewHandler(newTestUptimeService(&fakeSSM{value: "auto"}), staging, noopDispatcher(), "", "role-allowed")

				resp := h.handleCommand(context.Background(), commandInteraction(command, tt.member, "off"))

				denied := strings.Contains(contentOf(resp), "権限がありません")
				if denied != tt.wantDenied {
					t.Errorf("拒否された = %v, want %v (content=%q)", denied, tt.wantDenied, contentOf(resp))
				}
				// 拒否したのに書き込みまで進んでいれば、防御が効いていない。
				if tt.wantDenied && command == CommandNameStaging && staging.called() > 0 {
					t.Error("権限が無いのに SetState が呼ばれた")
				}
			})
		}
	}
}

// 権限があってもサービス未設定なら処理へ進まない（nil参照でpanicする）。
func TestHandler_ServiceUnset(t *testing.T) {
	member := &Member{Roles: []string{"role-allowed"}}

	tests := []struct {
		name    string
		handler *Handler
		command string
	}{
		{"prod: uptime未設定", NewHandler(nil, &stagingSetterStub{}, noopDispatcher(), "", "role-allowed"), CommandNameProd},
		{"staging: staging未設定", NewHandler(newTestUptimeService(&fakeSSM{}), nil, noopDispatcher(), "", "role-allowed"), CommandNameStaging},
		{"list: uptime未設定", NewHandler(nil, nil, noopDispatcher(), "", "role-allowed"), CommandNameProdUptimeList},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.handler.handleCommand(context.Background(), commandInteraction(tt.command, member, "on"))
			if !strings.Contains(contentOf(resp), "利用できません") {
				t.Errorf("未設定の案内が返っていない: %q", contentOf(resp))
			}
		})
	}
}

func TestHandler_RejectsUnknownState(t *testing.T) {
	member := &Member{Roles: []string{"role-allowed"}}
	ssm := &fakeSSM{value: "auto"}
	staging := &stagingSetterStub{}
	h := NewHandler(newTestUptimeService(ssm), staging, noopDispatcher(), "", "role-allowed")

	for _, command := range []string{CommandNameProd, CommandNameStaging} {
		t.Run(command, func(t *testing.T) {
			resp := h.handleCommand(context.Background(), commandInteraction(command, member, "restart"))

			if contentOf(resp) == "" || strings.Contains(contentOf(resp), "設定しました") {
				t.Errorf("不正な値を受け付けている: %q", contentOf(resp))
			}
			if ssm.putCalls > 0 || staging.called() > 0 {
				t.Error("不正な値なのに書き込みへ進んでいる")
			}
		})
	}
}

// コマンド名のルーティング。取り違えると /staging で本番が止まる。
func TestHandler_RoutesCommands(t *testing.T) {
	member := &Member{Roles: []string{"role-allowed"}}

	t.Run("stagingはstagingサービスへ", func(t *testing.T) {
		ssm := &fakeSSM{value: "auto"}
		staging := &stagingSetterStub{}
		h := NewHandler(newTestUptimeService(ssm), staging, noopDispatcher(), "", "role-allowed")

		h.handleCommand(context.Background(), commandInteraction(CommandNameStaging, member, "off"))

		if staging.called() != 1 {
			t.Errorf("staging SetState 呼び出し回数 = %d, want 1", staging.called())
		}
		if ssm.putCalls != 0 {
			t.Errorf("本番のSSMに書き込んでいる (putCalls=%d)", ssm.putCalls)
		}
	})

	t.Run("prodは本番のオーバーライドへ", func(t *testing.T) {
		ssm := &fakeSSM{value: "auto"}
		staging := &stagingSetterStub{}
		h := NewHandler(newTestUptimeService(ssm), staging, noopDispatcher(), "", "role-allowed")

		h.handleCommand(context.Background(), commandInteraction(CommandNameProd, member, "on"))

		if ssm.putValue != OverrideOn {
			t.Errorf("書き込まれた値 = %q, want %q", ssm.putValue, OverrideOn)
		}
		if staging.called() != 0 {
			t.Error("ステージングを触っている")
		}
	})

	t.Run("未知のコマンドは何もしない", func(t *testing.T) {
		ssm := &fakeSSM{value: "auto"}
		staging := &stagingSetterStub{}
		h := NewHandler(newTestUptimeService(ssm), staging, noopDispatcher(), "", "role-allowed")

		resp := h.handleCommand(context.Background(), commandInteraction("deploy", member, "on"))

		if !strings.Contains(contentOf(resp), "不明なコマンド") {
			t.Errorf("content = %q", contentOf(resp))
		}
		if ssm.putCalls != 0 || staging.called() != 0 {
			t.Error("未知のコマンドで書き込みへ進んでいる")
		}
	})
}

// Lambdaはレスポンスを返すと実行環境が凍結され、後追いの更新が走らない。
// deferred(type=5)で返すと「考え中」のまま残るため、応答は必ず確定形で返す。
func TestHandler_DoesNotDefer(t *testing.T) {
	member := &Member{Roles: []string{"role-allowed"}}
	h := NewHandler(newTestUptimeService(&fakeSSM{value: "auto"}), &stagingSetterStub{}, noopDispatcher(), "", "role-allowed")

	for _, command := range []string{CommandNameProd, CommandNameStaging, CommandNameProdUptimeList} {
		resp := h.handleCommand(context.Background(), commandInteraction(command, member, "on"))
		if resp.Type == ResponseTypeDeferredChannelMessageWithSource {
			t.Errorf("%s が deferred を返した", command)
		}
	}
}

// 署名検証はこのエンドポイント唯一の認証。素通しすると誰でも本番を止められる。
func TestHandler_VerifiesSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("鍵生成に失敗: %v", err)
	}
	publicKey := hex.EncodeToString(pub)
	body, _ := json.Marshal(Interaction{Type: InteractionTypePing})
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	validSig := hex.EncodeToString(ed25519.Sign(priv, append([]byte(ts), body...)))

	tests := []struct {
		name       string
		publicKey  string
		signature  string
		wantStatus int
	}{
		{"正しい署名", publicKey, validSig, http.StatusOK},
		{"署名が不正", publicKey, hex.EncodeToString(ed25519.Sign(priv, []byte("別のメッセージ"))), http.StatusUnauthorized},
		{"署名なし", publicKey, "", http.StatusUnauthorized},
		{"公開鍵が未設定なら誰も通さない", "", validSig, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(newTestUptimeService(&fakeSSM{}), &stagingSetterStub{}, noopDispatcher(), tt.publicKey, "role-allowed")

			status, resp := h.Handle(context.Background(), body, tt.signature, ts)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK && resp != nil {
				t.Error("認証に失敗したのに本文を返している")
			}
		})
	}
}

// DiscordはエンドポイントのPINGにPONGが返らないとURLの保存自体を拒否する。
func TestHandler_RespondsToPing(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	body, _ := json.Marshal(Interaction{Type: InteractionTypePing})
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := hex.EncodeToString(ed25519.Sign(priv, append([]byte(ts), body...)))

	h := NewHandler(newTestUptimeService(&fakeSSM{}), &stagingSetterStub{}, noopDispatcher(), hex.EncodeToString(pub), "role-allowed")

	status, resp := h.Handle(context.Background(), body, sig, ts)

	if status != http.StatusOK || resp == nil || resp.Type != ResponseTypePong {
		t.Errorf("status=%d resp=%+v, want 200 / Pong", status, resp)
	}
}

func TestHandler_RejectsOversizedBody(t *testing.T) {
	h := NewHandler(newTestUptimeService(&fakeSSM{}), &stagingSetterStub{}, noopDispatcher(), "pubkey", "role-allowed")

	status, resp := h.Handle(context.Background(), make([]byte, MaxBodyBytes+1), "sig", "ts")

	if status != http.StatusRequestEntityTooLarge || resp != nil {
		t.Errorf("status = %d, want %d", status, http.StatusRequestEntityTooLarge)
	}
}

func TestHandler_ModalSubmit(t *testing.T) {
	member := &Member{Roles: []string{"role-allowed"}}
	tomorrow := time.Now().In(jst()).AddDate(0, 0, 1).Format("2006-01-02")

	modal := func(customID, date string, m *Member) *Interaction {
		return &Interaction{
			Type:   InteractionTypeModalSubmit,
			Member: m,
			Data: &InteractionData{
				CustomID: customID,
				Components: []Component{{
					Type:       ComponentTypeActionRow,
					Components: []Component{{Type: ComponentTypeTextInput, CustomID: TextInputCustomIDDate, Value: date}},
				}},
			},
		}
	}

	tests := []struct {
		name        string
		interaction *Interaction
		wantSubstr  string
		wantPut     int
	}{
		{"正しい日付は追加される", modal(ModalCustomIDProdUptime, tomorrow, member), "追加しました", 1},
		{"不正な日付は弾く", modal(ModalCustomIDProdUptime, "9/1", member), "", 0},
		{"別のモーダルは無視", modal("other_modal", tomorrow, member), "不明な操作", 0},
		{"権限が無ければ拒否", modal(ModalCustomIDProdUptime, tomorrow, &Member{Roles: []string{"x"}}), "権限がありません", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ssm := &fakeSSM{value: ""}
			h := NewHandler(newTestUptimeService(ssm), &stagingSetterStub{}, noopDispatcher(), "", "role-allowed")

			resp := h.handleModalSubmit(context.Background(), tt.interaction)

			if tt.wantSubstr != "" && !strings.Contains(contentOf(resp), tt.wantSubstr) {
				t.Errorf("content = %q, want に %q を含む", contentOf(resp), tt.wantSubstr)
			}
			if ssm.putCalls != tt.wantPut {
				t.Errorf("putCalls = %d, want %d", ssm.putCalls, tt.wantPut)
			}
		})
	}
}

func TestHandler_hasAllowedRole(t *testing.T) {
	tests := []struct {
		name          string
		allowedRoleID string
		member        *Member
		want          bool
	}{
		{"一致するロールを持つ", "role-1", &Member{Roles: []string{"role-0", "role-1"}}, true},
		{"一致しない", "role-1", &Member{Roles: []string{"role-0"}}, false},
		{"Member が nil", "role-1", nil, false},
		// 未設定を「誰でも許可」にすると、環境変数の設定漏れが全開放になる
		{"許可ロール未設定なら誰も許可しない", "", &Member{Roles: []string{"role-1"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{allowedRoleID: tt.allowedRoleID}
			if got := h.hasAllowedRole(&Interaction{Member: tt.member}); got != tt.want {
				t.Errorf("hasAllowedRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverrideAppliedMessage(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{OverrideOn, "常時起動"},
		{OverrideOff, "常時停止"},
		{OverrideAuto, "日付リストに従う"},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := overrideAppliedMessage(tt.state); !strings.Contains(got, tt.want) {
				t.Errorf("overrideAppliedMessage(%q) = %q, want に %q を含む", tt.state, got, tt.want)
			}
		})
	}
}

func TestStagingStateAppliedMessage(t *testing.T) {
	if got := stagingStateAppliedMessage(StagingStateOn); !strings.Contains(got, "起動") {
		t.Errorf("on = %q", got)
	}
	if got := stagingStateAppliedMessage(StagingStateOff); !strings.Contains(got, "停止") {
		t.Errorf("off = %q", got)
	}
}

func TestJoinDates(t *testing.T) {
	tests := []struct {
		name  string
		dates []string
		want  string
	}{
		{"空", nil, "(なし)"},
		{"1件", []string{"2026-09-01"}, "2026-09-01"},
		{"複数", []string{"2026-09-01", "2026-09-02"}, "2026-09-01, 2026-09-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinDates(tt.dates); got != tt.want {
				t.Errorf("joinDates() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Discordのcontentは2000文字が上限。超えると送信自体が失敗して一覧が出せなくなる。
func TestJoinDates_TruncatesBeyondDiscordMessageLimit(t *testing.T) {
	dates := make([]string, 300)
	for i := range dates {
		dates[i] = fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)
	}

	got := joinDates(dates)

	if len(got) > 2000 {
		t.Errorf("長さ = %d, Discordの上限2000を超えている", len(got))
	}
	if !strings.Contains(got, "他") {
		t.Errorf("省略された件数が示されていない: %q", got)
	}
}

// SSMのエラーはARN等の内部情報を含みうるのでそのまま返さない。
func TestUserFacingErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"入力検証エラーはそのまま返す", errors.New("日付は YYYY-MM-DD で指定してください"), "日付は YYYY-MM-DD で指定してください"},
		{"ラップされた内部エラーは隠す", fmt.Errorf("SSM parameter更新に失敗しました: %w", errors.New("arn:aws:ssm:...")), "処理に失敗しました。時間を置いて再度お試しください。"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userFacingErrorMessage(tt.err); got != tt.want {
				t.Errorf("userFacingErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
