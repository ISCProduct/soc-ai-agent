package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/services/discord"

	"github.com/labstack/echo/v4"
)

// /prod は本番を止められるコマンドなので、ロール制限が唯一の防御になる。
// hasAllowedRole の単体テストだけでは、handleProdCommand から呼び忘れても検出できない。
func TestHandleProdCommand_Authorization(t *testing.T) {
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
			c := &DiscordInteractionController{allowedRoleID: "role-allowed"}
			rec, resp := invokeProdCommand(t, c, tt.member, "off")

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			denied := resp.Data != nil && strings.Contains(resp.Data.Content, "権限がありません")
			if denied != tt.wantDenied {
				t.Errorf("拒否された = %v, want %v (content=%q)", denied, tt.wantDenied, contentOf(resp))
			}
			// 拒否時はSSMに触れる非同期処理へ進んではならない。
			// uptimeService が nil なので、進んでいれば「利用できません」になる。
			if tt.wantDenied && resp.Type == discord.ResponseTypeDeferredChannelMessageWithSource {
				t.Error("権限が無いのに処理を開始している")
			}
		})
	}
}

// 権限があっても uptimeService 未設定なら処理へ進まない（goroutineがnil参照でpanicする）。
func TestHandleProdCommand_ServiceUnset(t *testing.T) {
	c := &DiscordInteractionController{allowedRoleID: "role-allowed"}
	_, resp := invokeProdCommand(t, c, &discord.Member{Roles: []string{"role-allowed"}}, "on")
	if !strings.Contains(contentOf(resp), "利用できません") {
		t.Errorf("content = %q, want 未設定の案内", contentOf(resp))
	}
	if resp.Type == discord.ResponseTypeDeferredChannelMessageWithSource {
		t.Error("uptimeService が無いのに処理を開始している")
	}
}

// on/off/auto 以外は受け付けない。SSMへ書く前に弾くこと。
func TestHandleProdCommand_RejectsUnknownState(t *testing.T) {
	c := &DiscordInteractionController{allowedRoleID: "role-allowed"}
	_, resp := invokeProdCommand(t, c, &discord.Member{Roles: []string{"role-allowed"}}, "stop")
	if !strings.Contains(contentOf(resp), "on / off / auto") {
		t.Errorf("content = %q, want 入力エラーの案内", contentOf(resp))
	}
	if resp.Type == discord.ResponseTypeDeferredChannelMessageWithSource {
		t.Error("不正な状態なのに処理を開始している")
	}
}

// 表示文言が実際の状態と入れ替わると、ユーザーは逆の状態だと思い込む。
func TestOverrideAppliedMessage(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{discord.OverrideOn, "常時起動"},
		{discord.OverrideOff, "常時停止"},
		{discord.OverrideAuto, "日付リストに従う"},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := overrideAppliedMessage(tt.state)
			if !strings.Contains(got, tt.want) {
				t.Errorf("overrideAppliedMessage(%q) = %q, want %q を含むこと", tt.state, got, tt.want)
			}
			// 逆の状態の文言が混ざっていないこと
			for _, other := range tests {
				if other.state != tt.state && strings.Contains(got, other.want) {
					t.Errorf("overrideAppliedMessage(%q) に %q が混ざっている: %q", tt.state, other.want, got)
				}
			}
		})
	}
}

func invokeProdCommand(t *testing.T, c *DiscordInteractionController, member *discord.Member, state string) (*httptest.ResponseRecorder, discord.InteractionResponse) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/discord/interactions", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	interaction := &discord.Interaction{
		Type:   discord.InteractionTypeApplicationCommand,
		Member: member,
		Data: &discord.InteractionData{
			Name:    discord.CommandNameProd,
			Options: []discord.CommandOption{{Name: discord.OptionNameState, Value: state}},
		},
	}
	if err := c.handleProdCommand(ctx, interaction); err != nil {
		t.Fatalf("handleProdCommand() error = %v", err)
	}

	var resp discord.InteractionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("レスポンスをデコードできない: %v (body=%s)", err, rec.Body.String())
	}
	return rec, resp
}

func contentOf(resp discord.InteractionResponse) string {
	if resp.Data == nil {
		return ""
	}
	return resp.Data.Content
}
