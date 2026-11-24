package company

import (
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/houjinbangou"
)

// 読み仮名を gBizINFO の保存済みの法人基本情報から作る。
//
// gBizINFO の法人基本情報（kana）は、同期のときにすでに gbiz_company_profiles へ保存している。
// それを使えば、法人番号公表サイトの Web-API（アプリケーションID）も、追加の API 呼び出しも要らない。
// 採用の条件は、法人番号公表サイト経由と同じ（登録の商号と一致すること。形式は全角カタカナ・法人格の読みなし）。

// MethodGBizInfo は、保存済みの gBizINFO のフリガナを使った。
const MethodGBizInfo = "gbizinfo"

// ReasonNoGBizProfile は、gBizINFO の法人基本情報が保存されていないとき。
const ReasonNoGBizProfile = "gBizINFO の情報が保存されていない"

// ReasonNumberMismatch は、企業の法人番号と gBizINFO の法人番号が違うとき。
const ReasonNumberMismatch = "gBizINFO の法人番号と一致しない"

// ReadingFromGBizProfile は、保存済みの gBizINFO の法人基本情報から読み仮名を作る。
// 採用できないときは Reading が空で、Reason に理由を入れる。誤った値を入れるより、空のままにする。
func ReadingFromGBizProfile(c *models.Company, p *models.GBizCompanyProfile) ReadingResult {
	if p == nil {
		return ReadingResult{Reason: ReasonNoGBizProfile}
	}
	// 法人番号の入力誤りで、別の法人の読みを入れないための確認。両方に値があるときだけ比べる。
	if cn := strings.TrimSpace(c.CorporateNumber); cn != "" && strings.TrimSpace(p.CorporateNumber) != "" && cn != strings.TrimSpace(p.CorporateNumber) {
		return ReadingResult{Reason: ReasonNumberMismatch}
	}
	// 商号が空、または一致しなければ採用しない（確認できないものは採用しない）。
	if strings.TrimSpace(p.Name) == "" ||
		houjinbangou.NormalizeCompanyName(p.Name) != houjinbangou.NormalizeCompanyName(c.Name) {
		return ReadingResult{Reason: ReasonNameMismatch}
	}
	reading, ok := NormalizeReading(p.NameKana)
	if !ok {
		return ReadingResult{Reason: ReasonNoReading}
	}
	if reading = stripLegalFormReading(reading); reading == "" {
		return ReadingResult{Reason: ReasonNoReading}
	}
	return ReadingResult{Reading: reading, Method: MethodGBizInfo}
}
