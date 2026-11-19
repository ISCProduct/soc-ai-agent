package piimask

import (
	"strings"
	"testing"
)

// すべて合成データ。実在の個人情報は使わない（公開リポジトリ）。
var subj = Subject{Name: "山田 太郎", Email: "taro.yamada@example.com", SchoolName: "架空情報専門学校"}

func mustMasker(t *testing.T, s Subject) *Masker {
	t.Helper()
	m, err := ForLLM(s)
	if err != nil {
		t.Fatalf("ForLLM: %v", err)
	}
	return m
}

func TestMask_RemovesIdentifiers(t *testing.T) {
	m := mustMasker(t, subj)
	tests := []struct {
		name string
		in   string
		gone []string // 出力に残ってはいけない語
		has  string   // 出力に含まれる記号
	}{
		{"氏名(辞書)", "私は山田太郎と申します。", []string{"山田太郎"}, "<NAME>"},
		{"氏名(姓名の間に空白)", "山田 太郎です", []string{"山田 太郎"}, "<NAME>"},
		{"メール(規則)", "連絡先は someone@example.org です", []string{"someone@example.org"}, "<EMAIL>"},
		{"メール(本人・ドメイン欠け)", "taro.yamada@example で連絡", []string{"taro.yamada"}, "<EMAIL>"},
		{"電話", "電話は090-1234-5678です", []string{"090-1234-5678"}, "<PHONE>"},
		{"電話(前に空白)", "連絡は a か 090-1234-5678 まで", []string{"090-1234-5678"}, "<PHONE>"},
		{"住所", "東京都港区六本木1-2-3に住んでいます", []string{"六本木1-2-3"}, "<ADDRESS>"},
		{"郵便番号", "〒 106-0032 の寮", []string{"106-0032"}, "<POSTAL>"},
		{"生年月日(見出し)", "生年月日: 1999年4月1日", []string{"1999年4月1日"}, "<BIRTHDATE>"},
		{"生年月日(生)", "平成11年4月1日生まれ", []string{"平成11年4月1日"}, "<BIRTHDATE>"},
		{"年齢", "満22歳です", []string{"満22歳"}, "<BIRTHDATE>"},
		{"URL", "作品は https://example.com/portfolio/taro にあります", []string{"example.com/portfolio"}, "<URL>"},
		{"SNS", "X は @taro_dev で発信", []string{"taro_dev"}, "<HANDLE>"},
		{"学校名(辞書)", "架空情報専門学校で学びました", []string{"架空情報専門学校"}, "<SCHOOL>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.Mask(tt.in)
			for _, g := range tt.gone {
				if strings.Contains(got, g) {
					t.Errorf("%q が残っている: %q", g, got)
				}
			}
			if !strings.Contains(got, tt.has) {
				t.Errorf("%q を含むはず: %q", tt.has, got)
			}
		})
	}
}

// レビューに必要な情報は残す。伏字にしすぎると、添削の質が落ちる。
func TestMask_KeepsReviewContent(t *testing.T) {
	m := mustMasker(t, subj)
	keep := []string{
		"株式会社サンプルのインターンで3か月、Webの開発を担当しました。", // 企業名は残す
		"令和4年3月 卒業",             // 日が無い年月は残す
		"2024年4月から2025年3月まで在籍",  // 在籍期間
		"売上を前年比120%に伸ばしました。",    // 数字
		"東京都の大学で情報を学びました。",      // 都道府県だけは残す
		"チームで10名のプロジェクトを進めました。", // 人数
	}
	for _, s := range keep {
		if got := m.Mask(s); got != s {
			t.Errorf("本文が変わった:\n  in : %q\n  out: %q", s, got)
		}
	}
}

func TestMask_EmptySubjectStillMasksRules(t *testing.T) {
	m := mustMasker(t, Subject{})
	got := m.Mask("090-1234-5678 / a.b@example.com")
	if strings.Contains(got, "090") || strings.Contains(got, "example.com") {
		t.Errorf("規則だけでも伏字になるはず: %q", got)
	}
}

// nil の Masker で黙って通さない。伏字の準備をせずに送るのを、実行時にも止める。
func TestMask_NilMaskerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil の Masker は panic するはず")
		}
	}()
	var m *Masker
	_ = m.Mask("山田太郎")
}

func TestIsContactFieldLine(t *testing.T) {
	yes := []string{"氏名 山田 太郎", "ふりがな やまだ たろう", "生年月日 1999年4月1日", "現住所 東京都港区", "電話 090-1234-5678", "E-mail taro@example.com", "性別 男", "連絡先：090-0000-0000", "TEL090-1234-5678", "メールアドレス taro@example.com"}
	for _, s := range yes {
		if !IsContactFieldLine(s) {
			t.Errorf("欄の行のはず: %q", s)
		}
	}
	no := []string{
		"",
		"志望動機",
		"電話応対のアルバイトで、お客様の要望を聞き取る力を身につけました。", // 見出しの語で始まるだけの文
		"住所を調べて配達する仕事をしていました。",              // 同上
		"電話 " + strings.Repeat("あ", 100),    // 長い行は本文とみなす
		"自己PR: チームで進める仕事が得意です",
	}
	for _, s := range no {
		if IsContactFieldLine(s) {
			t.Errorf("欄の行ではないはず: %q", s)
		}
	}
}
