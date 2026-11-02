package models_test

import (
	"strings"
	"testing"

	"Backend/internal/models"
)

// 設問キーは「同じ設問を同じものとして引き当てる」ためにある（#1531）。
// 揺れを吸収しすぎると別の設問が同一視され、吸収しなさすぎると差分が永久に出ない。
// どちらに倒しているかをテストで固定する。
func TestNormalizeESQuestion(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{"空白の有無", "学生時代に力を入れたことは？", "学生時代に力を入れたこと は ?", true},
		{"全角空白", "学生時代に力を入れたことは", "学生時代に　力を入れたことは", true},
		{"句読点の有無", "当社を志望する理由は？", "当社を志望する理由は", true},
		{"英字の大小", "Why our company?", "why our company", true},
		{"別の設問", "学生時代に力を入れたことは", "当社を志望する理由は", false},
		{"前後の空白", "  自己PRをどうぞ  ", "自己PRをどうぞ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ka := models.NormalizeESQuestion(tt.a)
			kb := models.NormalizeESQuestion(tt.b)
			if (ka == kb) != tt.same {
				t.Fatalf("same=%v を期待したが %q と %q になった", tt.same, ka, kb)
			}
		})
	}
}

// 列は varchar(255)。ルーン単位で切らないと多バイト文字の途中で切れて
// 別のキーになり、長い設問だけ差分が出なくなる。
func TestNormalizeESQuestion_TruncatesByRune(t *testing.T) {
	key := models.NormalizeESQuestion(strings.Repeat("あ", 300))
	if got := len([]rune(key)); got != 255 {
		t.Fatalf("255文字に丸めるべき: %d", got)
	}
	if len([]rune(key)) != len([]rune(string([]rune(key)))) {
		t.Fatal("多バイト文字の途中で切れている")
	}
}

// 設問が空のときはキーも空。空キー同士を「同一設問」として束ねると、
// 設問を入れなかった添削が全部ひとつの履歴に混ざる。呼び出し側で引き当てを飛ばす。
func TestNormalizeESQuestion_EmptyStaysEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "？？？"} {
		if got := models.NormalizeESQuestion(in); got != "" {
			t.Fatalf("入力 %q は空キーになるべき: %q", in, got)
		}
	}
}
