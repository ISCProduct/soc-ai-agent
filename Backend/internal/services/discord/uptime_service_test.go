package discord

import "testing"

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid date", "2026-09-01", false},
		{"valid date with whitespace", "  2026-09-01  ", false},
		{"wrong format slashes", "2026/09/01", true},
		{"wrong format order", "01-09-2026", true},
		{"invalid calendar date", "2026-02-30", true},
		{"empty", "", true},
		{"not a date at all", "tomorrow", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDate(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestFindComponentValue(t *testing.T) {
	components := []Component{
		{
			Type: ComponentTypeActionRow,
			Components: []Component{
				{Type: ComponentTypeTextInput, CustomID: TextInputCustomIDDate, Value: "2026-09-01"},
			},
		},
	}
	if got := FindComponentValue(components, TextInputCustomIDDate); got != "2026-09-01" {
		t.Errorf("FindComponentValue() = %q, want %q", got, "2026-09-01")
	}
	if got := FindComponentValue(components, "not_found"); got != "" {
		t.Errorf("FindComponentValue() for missing id = %q, want empty", got)
	}
}
