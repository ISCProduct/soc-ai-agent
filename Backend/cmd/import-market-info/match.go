package main

import (
	"strings"
)

// marketValue は、company_market_info に入れる（または入っている）値。
type marketValue struct {
	MarketType string `json:"market_type"`
	IsListed   bool   `json:"is_listed"`
	StockCode  string `json:"stock_code"`
}

// 判定の種類。
const (
	kindNew       = "new"         // 市場情報が無い企業に、新しく入れる
	kindDiff      = "diff"        // 既に値があり、取得結果と違う
	kindSame      = "same"        // 既に値があり、取得結果と同じ
	kindUnchecked = "unconfirmed" // EDINET に無い／上場区分が空（何も書かない）
)

type companyRef struct {
	ID              uint
	Name            string
	CorporateNumber string
	Existing        *marketValue // 既に市場情報がある場合（空の行は nil 扱いにして渡す）
}

type decision struct {
	CompanyID uint
	Name      string
	Kind      string
	Before    *marketValue
	After     *marketValue
	Note      string
}

// segmentToMarketType は、東証の「市場・商品区分」を prime / standard / growth にする。該当しなければ空（ETF・REIT 等）。
func segmentToMarketType(seg string) string {
	switch {
	case strings.HasPrefix(seg, "プライム"):
		return "prime"
	case strings.HasPrefix(seg, "スタンダード"):
		return "standard"
	case strings.HasPrefix(seg, "グロース"):
		return "growth"
	}
	return ""
}

// decide は、1 社の突き合わせ結果。法人番号の完全一致だけを使い、名前では結びつけない。
func decide(c companyRef, edinet map[string]edinetEntry, jpx map[string]string) decision {
	d := decision{CompanyID: c.ID, Name: c.Name, Before: c.Existing}
	e, ok := edinet[c.CorporateNumber]
	if !ok || !validCorpNumber(c.CorporateNumber) {
		d.Kind, d.Note = kindUnchecked, "EDINET に法人番号が無い"
		return d
	}
	var after marketValue
	switch e.Listing {
	case "上場":
		after.IsListed = true
		if len(e.SecCode) >= 4 {
			after.StockCode = e.SecCode[:4]
			after.MarketType = segmentToMarketType(jpx[after.StockCode])
		}
	case "非上場":
		after.MarketType = "unlisted"
	default:
		d.Kind, d.Note = kindUnchecked, "EDINET の上場区分が空"
		return d
	}
	d.After = &after
	switch {
	case c.Existing == nil:
		d.Kind = kindNew
	case *c.Existing == after:
		d.Kind = kindSame
	default:
		d.Kind = kindDiff
	}
	return d
}
