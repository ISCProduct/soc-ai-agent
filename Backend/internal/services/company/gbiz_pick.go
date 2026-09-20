package company

// gBizINFO の名称検索結果から、対象企業に当たるものを選ぶ。
//
// 検索は前方一致で、先頭が目的の法人とは限らない。実測では次のようになる。
//
//	「サイボウズ」   1件目 株式会社ＮＫサイボウズ / 2件目 サイボウズ・ラボ株式会社 / 3件目 サイボウズ株式会社
//	「トヨタ自動車」 1件目 軽井沢トヨタ自動車株式会社
//
// 先頭を無条件に採ると別会社の法人番号が企業レコードに入り、以降の同期が
// まるごと別会社の情報で上書きされる。商号の一致で選び、決まらなければ採らない。

import (
	"Backend/internal/services/gbizinfo"
	"Backend/internal/services/houjinbangou"
)

// pickGBizHit は商号が一致する候補を1件だけ選ぶ。決められなければ nil。
func pickGBizHit(hits []gbizinfo.GBizSearchResult, companyName, locationHint string) *gbizinfo.GBizSearchResult {
	want := houjinbangou.NormalizeCompanyName(companyName)
	if want == "" {
		return nil
	}

	var matched []gbizinfo.GBizSearchResult
	for _, h := range hits {
		if houjinbangou.NormalizeCompanyName(h.Name) == want {
			matched = append(matched, h)
		}
	}
	if len(matched) == 1 {
		if !houjinbangou.LocationMatches(matched[0].Location, locationHint) {
			return nil
		}
		return &matched[0]
	}
	if len(matched) == 0 {
		return nil
	}

	// 同名が複数。所在地で絞れるなら絞る。
	var narrowed []gbizinfo.GBizSearchResult
	for _, h := range matched {
		if houjinbangou.LocationMatches(h.Location, locationHint) {
			narrowed = append(narrowed, h)
		}
	}
	if len(narrowed) == 1 {
		return &narrowed[0]
	}
	return nil
}
