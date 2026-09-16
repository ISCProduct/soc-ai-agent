package houjinbangou

import (
	"context"
	"strings"

	"golang.org/x/text/width"
)

// legalForms は商号の比較時に取り除く法人格。
// 「トヨタ自動車」で検索してAPIが「トヨタ自動車株式会社」を返す、逆に社名に
// 「株式会社」が入っていて検索語に無い、といった差を吸収するために使う。
// 前株・後株のどちらにも現れるので、位置は問わず取り除く。
var legalForms = []string{
	"特定非営利活動法人",
	"一般社団法人", "一般財団法人", "公益社団法人", "公益財団法人",
	"独立行政法人", "国立大学法人", "公立大学法人", "社会福祉法人",
	"医療法人社団", "医療法人財団", "医療法人",
	"学校法人", "宗教法人",
	"株式会社", "有限会社", "合同会社", "合資会社", "合名会社",
}

// normalizeName は商号を比較用に正規化する。
// 法人格・空白・記号を落として、表記ゆれで一致判定を落とさないようにする。
func normalizeName(name string) string {
	// APIは全角で商号を返す（ＺＯＺＯ）が、DB側は半角のことがある（ZOZO）。
	// 幅を畳んでから比べないと同じ会社が一致しない。
	s := width.Fold.String(strings.TrimSpace(name))
	for _, form := range legalForms {
		s = strings.ReplaceAll(s, form, "")
	}
	// 全角・半角の空白と、社名に混じりがちな記号を落とす。
	for _, r := range []string{" ", "　", "・", "，", ",", "．", ".", "－", "-", "ー", "（", "）", "(", ")"} {
		s = strings.ReplaceAll(s, r, "")
	}
	return strings.ToLower(s)
}

// ResolveCorporateNumber は商号から法人番号を一意に特定する。
//
// locationHint には所在地（models.Company.Location など）を渡す。空でもよい。
// 日本では同一商号の別法人が普通に存在する（「株式会社ＺＯＺＯ」は2社ある）ため、
// 商号だけでは決まらないことが多い。その場合に都道府県で絞り込む。
//
// 一意に決まらなかった場合は (nil, nil) を返す。候補が複数あるときに先頭を採ると、
// 別会社の法人番号が企業レコードに入り、以降の gBizINFO 同期がまるごと別会社の
// 情報で上書きされる。誤った値を入れるより空のままにするほうが害が小さい。
func (c *Client) ResolveCorporateNumber(ctx context.Context, name, locationHint string) (*Corporation, error) {
	candidates, err := c.SearchByName(ctx, name)
	if err != nil {
		return nil, err
	}

	want := normalizeName(name)
	if want == "" {
		return nil, nil
	}

	// 登記が閉鎖されたものは最初に除く。
	var alive []Corporation
	for _, corp := range candidates {
		if !corp.IsClosed() {
			alive = append(alive, corp)
		}
	}

	// 検索語に法人格が入っているなら、法人格を含めた一致を先に試す。
	// 法人格を落として比べると「株式会社サイバーエージェント」と
	// 「有限会社サイバーエージェント」が同じ名前に潰れて、別法人なのに競合する。
	if hasLegalForm(name) {
		strict := normalizeKeepingLegalForm(name)
		var exact []Corporation
		for _, corp := range alive {
			if normalizeKeepingLegalForm(corp.Name) == strict {
				exact = append(exact, corp)
			}
		}
		if len(exact) == 1 {
			return &exact[0], nil
		}
		if len(exact) > 1 {
			alive = exact
		}
	}

	// 法人格を落とした一致。「トヨタ自動車」→「トヨタ自動車株式会社」を拾うため。
	// 前方一致検索なので「トヨタ自動車東京販売」のような別法人が紛れている。
	var matched []Corporation
	for _, corp := range alive {
		if normalizeName(corp.Name) == want {
			matched = append(matched, corp)
		}
	}

	if len(matched) == 1 {
		return &matched[0], nil
	}
	if len(matched) == 0 {
		return nil, nil
	}

	// 同名が複数。所在地の都道府県が分かるなら、それで絞れることがある。
	narrowed := narrowByLocation(matched, locationHint)
	if len(narrowed) == 1 {
		return &narrowed[0], nil
	}
	return nil, nil
}

// narrowByLocation は所在地の文字列に都道府県名が含まれる法人だけを残す。
// ヒントが空、または都道府県を判別できない場合は元の一覧をそのまま返す
// （絞れなかっただけで、呼び出し側は「一意に決まらない」として扱う）。
func narrowByLocation(corps []Corporation, locationHint string) []Corporation {
	hint := strings.TrimSpace(locationHint)
	if hint == "" {
		return corps
	}

	out := filterByField(corps, hint, func(c Corporation) string { return c.PrefectureName })
	if len(out) <= 1 {
		return out
	}
	// 都道府県だけでは足りないことがある（東京都に同名の株式会社と合同会社がある等）。
	// 市区町村まで絞れるなら絞る。
	return filterByField(out, hint, func(c Corporation) string { return c.CityName })
}

// filterByField は所在地ヒントに値が含まれる法人だけを残す。
// 1件も残らなければ「絞れなかった」として元の一覧を返す。
func filterByField(corps []Corporation, hint string, field func(Corporation) string) []Corporation {
	var out []Corporation
	for _, corp := range corps {
		v := strings.TrimSpace(field(corp))
		if v != "" && strings.Contains(hint, v) {
			out = append(out, corp)
		}
	}
	if len(out) == 0 {
		return corps
	}
	return out
}

// hasLegalForm は商号に法人格が含まれるかを返す。
func hasLegalForm(name string) bool {
	folded := width.Fold.String(name)
	for _, form := range legalForms {
		if strings.Contains(folded, form) {
			return true
		}
	}
	return false
}

// normalizeKeepingLegalForm は法人格を残したまま、幅・空白・記号だけを揃える。
func normalizeKeepingLegalForm(name string) string {
	s := width.Fold.String(strings.TrimSpace(name))
	for _, r := range []string{" ", "　", "・", "，", ",", "．", ".", "－", "-", "ー", "（", "）", "(", ")"} {
		s = strings.ReplaceAll(s, r, "")
	}
	return strings.ToLower(s)
}

// stripLegalForms は商号から法人格だけを取り除く（記号や空白は残す）。
// 国税庁APIの前方一致検索に渡す語を作るために使う。
func stripLegalForms(name string) string {
	s := strings.TrimSpace(name)
	for _, form := range legalForms {
		s = strings.ReplaceAll(s, form, "")
	}
	return strings.TrimSpace(s)
}
