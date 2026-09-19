package company

// gBizINFO 名称検索の結果選択テスト。
// 実行: cd Backend && go test ./internal/services/company/ -run TestPickGBizHit -v

import (
	"testing"

	"Backend/internal/services/gbizinfo"
)

func hit(number, name, location string) gbizinfo.GBizSearchResult {
	return gbizinfo.GBizSearchResult{CorporateNumber: number, Name: name, Location: location}
}

func TestPickGBizHit(t *testing.T) {
	tests := []struct {
		name     string
		company  string
		location string
		hits     []gbizinfo.GBizSearchResult
		want     string // 採用したい法人番号。空なら「採用しない」
	}{
		{
			// 実測。gBizINFO の前方一致は先頭が目的の法人とは限らない。
			name:     "先頭が別会社でも商号一致で選ぶ",
			company:  "サイボウズ株式会社",
			location: "東京都中央区",
			hits: []gbizinfo.GBizSearchResult{
				hit("1120001104811", "株式会社ＮＫサイボウズ", "東京都千代田区"),
				hit("3010001094715", "サイボウズ・ラボ株式会社", "東京都中央区"),
				hit("5010001072207", "サイボウズ株式会社", "東京都中央区日本橋"),
			},
			want: "5010001072207",
		},
		{
			// 実測。「トヨタ自動車」の先頭は軽井沢トヨタ自動車。
			name:     "前方一致で紛れた別法人を採用しない",
			company:  "トヨタ自動車株式会社",
			location: "愛知県豊田市",
			hits: []gbizinfo.GBizSearchResult{
				hit("1100001008774", "軽井沢トヨタ自動車株式会社", "長野県北佐久郡"),
				hit("1180301018771", "トヨタ自動車株式会社", "愛知県豊田市トヨタ町"),
			},
			want: "1180301018771",
		},
		{
			name:     "法人格の位置が違っても一致させる",
			company:  "メルカリ",
			location: "東京都港区",
			hits:     []gbizinfo.GBizSearchResult{hit("6010701027558", "株式会社メルカリ", "東京都港区六本木")},
			want:     "6010701027558",
		},
		{
			name:     "商号が一致しなければ採用しない",
			company:  "存在しない商事株式会社",
			location: "東京都",
			hits: []gbizinfo.GBizSearchResult{
				hit("1111111111111", "存在しない商事ホールディングス株式会社", "東京都港区"),
			},
			want: "",
		},
		{
			name:     "同名が複数でも所在地で絞れれば採用する",
			company:  "テスト商事",
			location: "大阪府大阪市北区",
			hits: []gbizinfo.GBizSearchResult{
				hit("1111111111111", "テスト商事株式会社", "東京都港区"),
				hit("2222222222222", "テスト商事株式会社", "大阪府大阪市北区梅田"),
			},
			want: "2222222222222",
		},
		{
			name:     "同名が複数で所在地でも絞れなければ採用しない",
			company:  "テスト商事",
			location: "",
			hits: []gbizinfo.GBizSearchResult{
				hit("1111111111111", "テスト商事株式会社", "東京都港区"),
				hit("2222222222222", "テスト商事株式会社", "大阪府大阪市北区"),
			},
			want: "",
		},
		{
			// 候補が1件でも、所在地が食い違えば別会社の可能性がある。
			name:     "1件でも都道府県が食い違えば採用しない",
			company:  "テスト商事",
			location: "北海道札幌市",
			hits:     []gbizinfo.GBizSearchResult{hit("1111111111111", "テスト商事株式会社", "沖縄県那覇市")},
			want:     "",
		},
		{
			name:     "所在地ヒントが無ければ商号一致だけで採用する",
			company:  "テスト商事",
			location: "",
			hits:     []gbizinfo.GBizSearchResult{hit("1111111111111", "テスト商事株式会社", "東京都港区")},
			want:     "1111111111111",
		},
		{
			name:     "候補ゼロ",
			company:  "テスト商事",
			location: "",
			hits:     nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickGBizHit(tt.hits, tt.company, tt.location)

			if tt.want == "" {
				if got != nil {
					t.Errorf("採用しないはずが %s (%s) を選んだ", got.CorporateNumber, got.Name)
				}
				return
			}
			if got == nil {
				t.Fatalf("%s を採用すべきだが nil", tt.want)
			}
			if got.CorporateNumber != tt.want {
				t.Errorf("got %s (%s), want %s", got.CorporateNumber, got.Name, tt.want)
			}
		})
	}
}
