package models

import "testing"

func TestNormalizeEmployeeCountBasis(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"consolidated", EmployeeCountBasisConsolidated},
		{"連結", EmployeeCountBasisConsolidated},
		{"standalone", EmployeeCountBasisStandalone},
		{"単体", EmployeeCountBasisStandalone},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := NormalizeEmployeeCountBasis(tt.in); got != tt.want {
			t.Errorf("NormalizeEmployeeCountBasis(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatEmployeeCount(t *testing.T) {
	if got := FormatEmployeeCount(101800, "consolidated"); got != "101800名（連結）" {
		t.Errorf("got %q", got)
	}
	if got := FormatEmployeeCount(21934, "単体"); got != "21934名（単体）" {
		t.Errorf("got %q", got)
	}
	if got := FormatEmployeeCount(100, ""); got != "100名" {
		t.Errorf("got %q", got)
	}
	if got := FormatEmployeeCount(0, "consolidated"); got != "" {
		t.Errorf("got %q", got)
	}
}
