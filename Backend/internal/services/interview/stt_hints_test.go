package interview

import (
	"strings"
	"testing"
)

// 「御社」は mini が補助語なしだと8回中0回しか正しく取れず、全て「本社」になった。
// 面接では意味が変わるため、企業情報の有無に関わらず必ず含める。
func TestBuildSTTHints_AlwaysIncludesOnsha(t *testing.T) {
	tests := []struct {
		name                                        string
		companyName, reading, position, companyInfo string
	}{
		{"全て空", "", "", "", ""},
		{"企業名のみ", "サンプル商事", "", "", ""},
		{"全部あり", "サンプル商事", "サンプルショウジ", "エンジニア", "Go と AWS を使っています"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSTTHints(tt.companyName, tt.reading, tt.position, tt.companyInfo)
			if !strings.Contains(got, "御社") {
				t.Errorf("補助語に「御社」が無い: %q", got)
			}
		})
	}
}

// 企業情報が空でも従来どおり動くこと（受け入れ条件）。
func TestBuildSTTHints_EmptyContext(t *testing.T) {
	got := BuildSTTHints("", "", "", "")
	if got == "" {
		t.Error("空文字を返している。「御社」だけは常に必要")
	}
	if strings.Contains(got, ",,") || strings.HasPrefix(got, ",") {
		t.Errorf("空要素が混ざっている: %q", got)
	}
}

func TestBuildSTTHints_IncludesCompanyContext(t *testing.T) {
	got := BuildSTTHints("株式会社サンプルソフト", "サンプルソフト", "バックエンドエンジニア", "")
	for _, want := range []string{"株式会社サンプルソフト", "サンプルソフト", "バックエンドエンジニア"} {
		if !strings.Contains(got, want) {
			t.Errorf("補助語に %q が無い: %q", want, got)
		}
	}
}

// 企業情報の文章をそのまま渡すと、補助語ではなく「続きの文脈」として
// 扱われ認識が引きずられる。技術用語だけを抜き出す。
func TestBuildSTTHints_ExtractsOnlyTechTerms(t *testing.T) {
	info := "当社は業務システムの受託開発を行っています。Go と TypeScript、AWS、MySQL を使っています。"
	got := BuildSTTHints("", "", "", info)
	for _, want := range []string{"Go", "TypeScript", "AWS", "MySQL"} {
		if !strings.Contains(got, want) {
			t.Errorf("技術用語 %q が抽出されていない: %q", want, got)
		}
	}
	// 日本語の一般語は補助語にしない
	for _, ng := range []string{"業務システム", "受託開発", "行っています"} {
		if strings.Contains(got, ng) {
			t.Errorf("一般語 %q が混ざっている: %q", ng, got)
		}
	}
}

// 長すぎる補助語は効果が薄れ、費用も増える。
//
// 実際の上限は技術用語の抽出側（MaxTechTerms）で決まる。
// 企業名・読み・職種は各1語なので、全体は最大でも 1+3+8 = 12語。
func TestBuildSTTHints_LimitsLength(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("Term")
		sb.WriteByte(byte('A' + i%26))
		sb.WriteString(" ")
	}
	got := BuildSTTHints("会社名", "カイシャメイ", "職種名", sb.String())
	// 「御社」+ 企業名・読み・職種の3語 + 技術用語 MaxTechTerms 語が上限
	if n := len(strings.Split(got, ", ")); n > 1+3+MaxTechTerms {
		t.Errorf("補助語が %d 語。上限 %d を超えている: %q", n, 1+3+MaxTechTerms, got)
	}
}

// 技術用語の抽出そのものに上限があること。
func TestExtractTechTerms_Limit(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString("Lang")
		sb.WriteByte(byte('A' + i%26))
		sb.WriteString(" ")
	}
	if got := len(extractTechTerms(sb.String())); got > MaxTechTerms {
		t.Errorf("技術用語 %d 語。上限 %d を超えている", got, MaxTechTerms)
	}
}

// 重複を出すと補助語の枠を無駄に使う。
func TestBuildSTTHints_Deduplicates(t *testing.T) {
	got := BuildSTTHints("サンプル", "サンプル", "サンプル", "サンプル")
	if strings.Count(got, "サンプル") != 1 {
		t.Errorf("重複している: %q", got)
	}
}

// 1文字の語は誤検出を招くだけで補助にならない。
func TestBuildSTTHints_SkipsSingleChar(t *testing.T) {
	got := BuildSTTHints("A", "い", "", "")
	for _, ng := range []string{"A,", "い,"} {
		if strings.Contains(got, ng) {
			t.Errorf("1文字の語が入っている: %q", got)
		}
	}
}

// 実行ごとに順序が変わると認識結果の再現性が落ちる。
func TestBuildSTTHints_IsStable(t *testing.T) {
	info := "Go TypeScript AWS MySQL Docker Kubernetes"
	first := BuildSTTHints("会社", "", "", info)
	for i := 0; i < 5; i++ {
		if got := BuildSTTHints("会社", "", "", info); got != first {
			t.Fatalf("実行ごとに結果が変わる:\n1回目: %q\n%d回目: %q", first, i+2, got)
		}
	}
}
