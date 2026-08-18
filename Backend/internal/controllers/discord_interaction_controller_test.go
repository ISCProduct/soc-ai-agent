package controllers

import (
	"Backend/internal/services/discord"
	"fmt"
	"strings"
	"testing"
)

func TestDiscordInteractionController_hasAllowedRole(t *testing.T) {
	tests := []struct {
		name          string
		allowedRoleID string
		member        *discord.Member
		want          bool
	}{
		{"member has allowed role", "role-123", &discord.Member{Roles: []string{"role-999", "role-123"}}, true},
		{"member lacks allowed role", "role-123", &discord.Member{Roles: []string{"role-999"}}, false},
		{"no member (e.g. DM)", "role-123", nil, false},
		{"allowedRoleID unset (fail closed)", "", &discord.Member{Roles: []string{"role-123"}}, false},
		{"member with no roles", "role-123", &discord.Member{Roles: []string{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DiscordInteractionController{allowedRoleID: tt.allowedRoleID}
			interaction := &discord.Interaction{Member: tt.member}
			if got := c.hasAllowedRole(interaction); got != tt.want {
				t.Errorf("hasAllowedRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinDates(t *testing.T) {
	tests := []struct {
		name  string
		dates []string
		want  string
	}{
		{"empty", nil, "(なし)"},
		{"single", []string{"2026-09-01"}, "2026-09-01"},
		{"multiple", []string{"2026-09-01", "2026-09-02"}, "2026-09-01, 2026-09-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinDates(tt.dates); got != tt.want {
				t.Errorf("joinDates() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinDates_TruncatesBeyondDiscordMessageLimit(t *testing.T) {
	dates := make([]string, 300)
	for i := range dates {
		dates[i] = fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)
	}

	got := joinDates(dates)

	if len(got) >= 2000 {
		t.Fatalf("joinDates() length = %d, must stay well under Discord's 2000 char content limit", len(got))
	}
	if !strings.Contains(got, "件)") {
		t.Errorf("joinDates() = %q, want truncation suffix indicating remaining count", got)
	}
}

func TestUserFacingErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"plain validation error is shown as-is", fmt.Errorf("過去の日付は指定できません（今日: 2026-08-18）"), "過去の日付は指定できません（今日: 2026-08-18）"},
		{"wrapped internal error is replaced", fmt.Errorf("SSM parameter更新に失敗しました: %w", fmt.Errorf("AccessDenied: arn:aws:iam::123456789012:user/foo")), "処理に失敗しました。時間を置いて再度お試しください。"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userFacingErrorMessage(tt.err); got != tt.want {
				t.Errorf("userFacingErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
