package gbizinfo

import (
	"testing"
)

func TestSplitClearAgencyNames(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"空文字", "", nil},
		{"空白のみ", "   ", nil},
		{"単一名", "株式会社テスト商事", []string{"株式会社テスト商事"}},
		{"スラッシュ区切り", "株式会社テスト商事／株式会社サンプル技研", []string{"株式会社テスト商事", "株式会社サンプル技研"}},
		{"カンマ区切り", "株式会社テスト商事、株式会社サンプル技研", []string{"株式会社テスト商事", "株式会社サンプル技研"}},
		{"重複除去", "株式会社テスト商事/株式会社テスト商事", []string{"株式会社テスト商事"}},
		{"大小文字違いの重複除去", "株式会社ABC/株式会社abc", []string{"株式会社ABC"}},
		{"不明瞭な名前は除外", "A/株式会社テスト商事", []string{"株式会社テスト商事"}},
		{"パイプ区切り", "株式会社テスト商事｜株式会社サンプル技研", []string{"株式会社テスト商事", "株式会社サンプル技研"}},
		{"セミコロン区切り", "株式会社テスト商事；株式会社サンプル技研", []string{"株式会社テスト商事", "株式会社サンプル技研"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitClearAgencyNames(tt.raw)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("splitClearAgencyNames(%q) nil-ness mismatch: got %v, want %v", tt.raw, got, tt.want)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitClearAgencyNames(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseJointSignatures(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"空文字", "", nil},
		{"空白のみ", "  ", nil},
		{"有効なJSON配列", `["Alice","Bob"]`, []string{"Alice", "Bob"}},
		{"不正なJSON", "not json", []string{}},
		{"空配列", "[]", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJointSignatures(tt.raw)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("parseJointSignatures(%q) nil-ness mismatch: got %v, want %v", tt.raw, got, tt.want)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseJointSignatures(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMarshalJointSignatures(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  string
	}{
		{"nilスライス", nil, ""},
		{"空スライス", []string{}, ""},
		{"空白のみ含むスライス", []string{"  ", ""}, ""},
		{"有効な名前", []string{"Alice", "Bob"}, `["Alice","Bob"]`},
		{"空白混在", []string{" Alice ", "", "Bob"}, `["Alice","Bob"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalJointSignatures(tt.names)
			if got != tt.want {
				t.Errorf("marshalJointSignatures(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}
