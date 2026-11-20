package company

import (
	"testing"

	"Backend/internal/models"
)

// 合成データ（実在の企業ではない）。
func TestReadingFromGBizProfile(t *testing.T) {
	company := func(name, number string) *models.Company {
		return &models.Company{Name: name, CorporateNumber: number}
	}
	profile := func(name, kana, number string) *models.GBizCompanyProfile {
		return &models.GBizCompanyProfile{Name: name, NameKana: kana, CorporateNumber: number}
	}
	tests := []struct {
		name       string
		c          *models.Company
		p          *models.GBizCompanyProfile
		want       string
		wantReason string
	}{
		{"ひらがなのフリガナをカタカナにする", company("株式会社サンプル商事", "1234567890123"), profile("株式会社サンプル商事", "さんぷるしょうじ", "1234567890123"), "サンプルショウジ", ""},
		{"法人格の読みは除く", company("サンプル商事株式会社", ""), profile("サンプル商事株式会社", "さんぷるしょうじかぶしきがいしゃ", ""), "サンプルショウジ", ""},
		// 法人番号公表サイト経由（ResolveNameReading）と同じ厳密さ。確認できないものは採用しない。
		{"法人格の表記が違う商号は同じとみなさない", company("(株)サンプル商事", ""), profile("株式会社サンプル商事", "さんぷるしょうじ", ""), "", ReasonNameMismatch},
		{"保存が無ければ採用しない", company("株式会社サンプル商事", ""), nil, "", ReasonNoGBizProfile},
		{"商号が違えば採用しない", company("株式会社サンプル商事", ""), profile("株式会社別の会社", "べつのかいしゃ", ""), "", ReasonNameMismatch},
		{"gBizINFO の商号が空なら確認できないので採用しない", company("株式会社サンプル商事", ""), profile("", "さんぷるしょうじ", ""), "", ReasonNameMismatch},
		{"法人番号が違えば採用しない", company("株式会社サンプル商事", "1111111111111"), profile("株式会社サンプル商事", "さんぷるしょうじ", "2222222222222"), "", ReasonNumberMismatch},
		{"企業の法人番号が空なら番号は比べない", company("株式会社サンプル商事", ""), profile("株式会社サンプル商事", "さんぷるしょうじ", "2222222222222"), "サンプルショウジ", ""},
		{"フリガナが空なら採用しない", company("株式会社サンプル商事", ""), profile("株式会社サンプル商事", "", ""), "", ReasonNoReading},
		{"読みでない文字（漢字）を含めば採用しない", company("株式会社サンプル商事", ""), profile("株式会社サンプル商事", "サンプル商事", ""), "", ReasonNoReading},
		{"法人格の読みだけなら採用しない", company("株式会社サンプル商事", ""), profile("株式会社サンプル商事", "かぶしきがいしゃ", ""), "", ReasonNoReading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReadingFromGBizProfile(tt.c, tt.p)
			if got.Reading != tt.want || got.Reason != tt.wantReason {
				t.Fatalf("got (reading=%q, reason=%q), want (%q, %q)", got.Reading, got.Reason, tt.want, tt.wantReason)
			}
			if tt.want != "" && got.Method != MethodGBizInfo {
				t.Fatalf("method = %q, want %q", got.Method, MethodGBizInfo)
			}
		})
	}
}
