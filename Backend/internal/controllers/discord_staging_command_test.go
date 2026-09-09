package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/internal/services/discord"

	"github.com/labstack/echo/v4"
)

type stagingSetterStub struct {
	mu     sync.Mutex
	values []string
}

func (s *stagingSetterStub) SetState(_ context.Context, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values = append(s.values, v)
	return nil
}

func (s *stagingSetterStub) called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.values)
}

// /staging は開発チーム全体の環境を止められるので、
// ロール制限が唯一の防御になる。handleStagingCommand から
// 呼び忘れても hasAllowedRole の単体テストでは検出できない。
func TestHandleStagingCommand_Authorization(t *testing.T) {
	tests := []struct {
		name       string
		member     *discord.Member
		wantDenied bool
	}{
		{"許可ロールを持つ", &discord.Member{Roles: []string{"role-allowed"}}, false},
		{"別のロールしか持たない", &discord.Member{Roles: []string{"role-other"}}, true},
		{"ロールなし", &discord.Member{Roles: []string{}}, true},
		{"DMなど Member が無い", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stagingSetterStub{}
			c := &DiscordInteractionController{allowedRoleID: "role-allowed", stagingService: stub}
			rec, resp := invokeStagingCommand(t, c, tt.member, "off")

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			denied := resp.Data != nil && strings.Contains(resp.Data.Content, "権限がありません")
			if denied != tt.wantDenied {
				t.Errorf("拒否された = %v, want %v", denied, tt.wantDenied)
			}
			// 拒否時はSSMへ書き込む非同期処理へ進んではならない
			if tt.wantDenied && stub.called() != 0 {
				t.Errorf("拒否したのに書き込もうとした: %v", stub.values)
			}
		})
	}
}

// サービス未設定なら「利用できません」と返し、落ちないこと。
func TestHandleStagingCommand_ServiceUnset(t *testing.T) {
	c := &DiscordInteractionController{allowedRoleID: "role-allowed"}
	_, resp := invokeStagingCommand(t, c, &discord.Member{Roles: []string{"role-allowed"}}, "on")
	if resp.Data == nil || !strings.Contains(resp.Data.Content, "利用できません") {
		t.Errorf("未設定時の応答 = %q", contentOf(resp))
	}
}

// on/off 以外は受け付けない。auto は本番用の概念で、ステージングには無い。
func TestHandleStagingCommand_RejectsUnknownState(t *testing.T) {
	for _, state := range []string{"auto", "yes", ""} {
		t.Run(state, func(t *testing.T) {
			stub := &stagingSetterStub{}
			c := &DiscordInteractionController{allowedRoleID: "role-allowed", stagingService: stub}
			_, resp := invokeStagingCommand(t, c, &discord.Member{Roles: []string{"role-allowed"}}, state)
			if resp.Data == nil || !strings.Contains(resp.Data.Content, "on / off") {
				t.Errorf("不正な状態 %q の応答 = %q", state, contentOf(resp))
			}
			if stub.called() != 0 {
				t.Errorf("不正な値を書き込もうとした: %v", stub.values)
			}
		})
	}
}

func TestStagingStateAppliedMessage(t *testing.T) {
	if got := stagingStateAppliedMessage(discord.StagingStateOn); !strings.Contains(got, "起動") {
		t.Errorf("on の文言 = %q", got)
	}
	if got := stagingStateAppliedMessage(discord.StagingStateOff); !strings.Contains(got, "停止") {
		t.Errorf("off の文言 = %q", got)
	}
	// 起動と停止で同じ文言だと、押し間違えに気づけない
	if stagingStateAppliedMessage(discord.StagingStateOn) == stagingStateAppliedMessage(discord.StagingStateOff) {
		t.Error("on と off で文言が同じ")
	}
}

func invokeStagingCommand(t *testing.T, c *DiscordInteractionController, member *discord.Member, state string) (*httptest.ResponseRecorder, discord.InteractionResponse) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/discord/interactions", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	interaction := &discord.Interaction{
		Type:   discord.InteractionTypeApplicationCommand,
		Member: member,
		Data: &discord.InteractionData{
			Name:    discord.CommandNameStaging,
			Options: []discord.CommandOption{{Name: discord.OptionNameState, Value: state}},
		},
	}
	if err := c.handleStagingCommand(ctx, interaction); err != nil {
		t.Fatalf("handleStagingCommand() error = %v", err)
	}

	var resp discord.InteractionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスをデコードできない: %v (body=%s)", err, rec.Body.String())
	}
	return rec, resp
}

// handleCommand から /staging へ振り分けられること。
// ハンドラ単体のテストだけでは、ルーティングを繋ぎ忘れても検出できず、
// 「不明なコマンドです」が返るだけで気づけない。
func TestHandleCommand_RoutesStaging(t *testing.T) {
	stub := &stagingSetterStub{}
	c := &DiscordInteractionController{allowedRoleID: "role-allowed", stagingService: stub}

	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/discord/interactions", nil), rec)

	interaction := &discord.Interaction{
		Type:   discord.InteractionTypeApplicationCommand,
		Member: &discord.Member{Roles: []string{"role-other"}}, // 権限なしで止める（SSMへ行かせない）
		Data: &discord.InteractionData{
			Name:    discord.CommandNameStaging,
			Options: []discord.CommandOption{{Name: discord.OptionNameState, Value: "off"}},
		},
	}
	if err := c.handleCommand(ctx, interaction); err != nil {
		t.Fatalf("handleCommand() error = %v", err)
	}

	var resp discord.InteractionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("デコードできない: %v", err)
	}
	content := contentOf(resp)
	if strings.Contains(content, "不明なコマンド") {
		t.Errorf("/staging がルーティングされていない: %q", content)
	}
	if !strings.Contains(content, "権限がありません") {
		t.Errorf("/staging ハンドラへ届いていない: %q", content)
	}
}
