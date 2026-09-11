package util

import "testing"

func TestNormalizeSchoolName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"情報科学専門学校", "情報科学専門学校"},
		{"  情報科学専門学校  ", "情報科学専門学校"},
		{"学校法人情報科学専門学校", "情報科学専門学校"},
		{"情報科学 専門学校", "情報科学専門学校"},
		{"情報科学　専門学校", "情報科学専門学校"},
	}
	for _, tt := range tests {
		got := NormalizeSchoolName(tt.in)
		if got != tt.want {
			t.Fatalf("NormalizeSchoolName(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}
