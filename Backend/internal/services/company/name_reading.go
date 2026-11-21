package company

import (
	"context"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

// 企業名の読み仮名（companies.name_reading）を、公的データ（法人番号公表サイト）から埋める（SOCAIAGENT-670 の関連）。
//
// これまで name_reading を書き込む処理が無く、実データでは 0% だった。面接では DB に無いと AI（モデルの知識）で
// 毎回補完しており、遅延・費用・読み間違いの原因になっていた。AI による推測は使わず、公的データだけを採用する。
// 採用できない場合は、誤った値を入れるより空のままにする。

// 読み仮名の取得方法。
const (
	MethodCorporateNumber = "corporate_number" // 登録済みの法人番号で引いた
	MethodNameMatch       = "name_match"       // 商号と所在地から、法人を一意に特定した
)

// 読み仮名を採用しなかった理由。
const (
	ReasonNotFound     = "法人番号が見つからない"
	ReasonClosed       = "登記が閉鎖されている"
	ReasonNameMismatch = "登録の商号と一致しない"
	ReasonNotResolved  = "法人を一意に特定できない"
	ReasonNoReading    = "フリガナが無い、または読みとして使えない"
)

// CorporationFinder は法人番号公表サイトの検索（houjinbangou.Client）。
type CorporationFinder interface {
	FindByNumber(ctx context.Context, corporateNumber string) (*houjinbangou.Corporation, error)
	ResolveCorporateNumber(ctx context.Context, name, locationHint string) (*houjinbangou.Corporation, error)
}

// ReadingResult は読み仮名の取得結果。Reading が空なら採用しない（Reason に理由）。
type ReadingResult struct {
	Reading string
	Method  string
	Reason  string
}

// NormalizeReading は、読み仮名として使える形（全角カタカナ）に整える。
// 半角カタカナは全角に、ひらがなはカタカナにする。漢字・英字を含むものは、読みではないので使えない。
func NormalizeReading(raw string) (string, bool) {
	s := strings.TrimSpace(norm.NFKC.String(raw))
	if s == "" {
		return "", false
	}
	var b strings.Builder
	hasKana := false
	for _, r := range s {
		switch {
		case r >= 'ぁ' && r <= 'ゖ': // ひらがな → カタカナ
			b.WriteRune(r + 0x60)
			hasKana = true
		case r >= 'ァ' && r <= 'ヺ':
			b.WriteRune(r)
			hasKana = true
		case r == 'ー' || r == '・' || r == ' ' || r == '　':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			return "", false
		}
	}
	if !hasKana {
		return "", false
	}
	return b.String(), true
}

// ResolveNameReading は、企業の読み仮名を法人番号公表サイトから引く。
//   - 法人番号が登録されていれば、その番号で引く。登録の商号と一致しなければ採用しない（番号の入力誤りを避ける）。
//   - 無ければ、商号と所在地で一意に特定できたときだけ採用する。
//
// 登記が閉鎖された法人は採用しない。取得の失敗（通信・設定）は、「見つからない」と区別して error で返す。
func ResolveNameReading(ctx context.Context, finder CorporationFinder, c *models.Company) (ReadingResult, error) {
	var corp *houjinbangou.Corporation
	var method string
	var err error

	if number := strings.TrimSpace(c.CorporateNumber); number != "" {
		method = MethodCorporateNumber
		if corp, err = finder.FindByNumber(ctx, number); err != nil {
			return ReadingResult{}, err
		}
		if corp == nil {
			return ReadingResult{Reason: ReasonNotFound}, nil
		}
		if houjinbangou.NormalizeCompanyName(corp.Name) != houjinbangou.NormalizeCompanyName(c.Name) {
			return ReadingResult{Reason: ReasonNameMismatch}, nil
		}
	} else {
		method = MethodNameMatch
		if corp, err = finder.ResolveCorporateNumber(ctx, c.Name, c.Location); err != nil {
			return ReadingResult{}, err
		}
		if corp == nil {
			return ReadingResult{Reason: ReasonNotResolved}, nil
		}
	}

	if corp.IsClosed() {
		return ReadingResult{Reason: ReasonClosed}, nil
	}
	reading, ok := NormalizeReading(corp.Furigana)
	if !ok {
		return ReadingResult{Reason: ReasonNoReading}, nil
	}
	return ReadingResult{Reading: reading, Method: method}, nil
}
