package controllers

import (
	"Backend/internal/services/discord"
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
