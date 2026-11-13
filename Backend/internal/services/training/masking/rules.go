package masking

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

// 項目の Key。Dictionary のキーに使う。
const (
	KeyName    = "name"
	KeyEmail   = "email"
	KeySchool  = "school"
	KeyCompany = "company"
	KeyPhone   = "phone"
	KeyAddress = "address"
)

// 置き換えの記号。<EMAIL> と <PHONE> は RAG 側の mask_pii（rag/training/export_training_data.py）と揃えている。
// 氏名は RAG 側では <PERSON> だが、#1834 の設計で決めた <NAME> を使う。
const (
	TokenName    = "<NAME>"
	TokenEmail   = "<EMAIL>"
	TokenSchool  = "<SCHOOL>"
	TokenCompany = "<COMPANY>"
	TokenPhone   = "<PHONE>"
	TokenAddress = "<ADDRESS>"
)

// DefaultRules は学習データの書き出しで置き換える項目。
// 項目を足すときは、ここに定義を1つ足す（漏れの検査のテストに例文も足すこと。無ければテストが落ちる）。
// 呼び出すたびに新しいスライスを返すので、呼び出し側で append してもほかに影響しない。
func DefaultRules() []Rule {
	return []Rule{
		{Key: KeyName, Token: TokenName, Method: MethodDictionary},
		{Key: KeyEmail, Token: TokenEmail, Method: MethodPattern, Pattern: emailPattern},
		{Key: KeySchool, Token: TokenSchool, Method: MethodDictionary},
		{Key: KeyCompany, Token: TokenCompany, Method: MethodDictionary},
		{Key: KeyPhone, Token: TokenPhone, Method: MethodPattern, Pattern: phonePattern, Accept: notInsideDigits},
		{Key: KeyAddress, Token: TokenAddress, Method: MethodPattern, Pattern: addressPattern},
	}
}

// emailPattern はメールアドレス。全角の英数字・「＠」「．」も拾う（音声認識や入力で全角になることがあるため）。
// 末尾はトップレベルドメインとして英字2文字以上を求める。「a@b」のような、メールでない「@」の使い方は拾わない。
// ローカル部は RFC 5322 の atext の記号（!#$%&'*+/=?^_`{|}~-）と「.」を含める。含めないと
// 「john!doe@example.com」が「john!<EMAIL>」になり、ローカル部の一部が残るため。
// ローカル部とドメインに日本語・空白を含めないので、「メールはtaro@example.comです」の前後の日本語は残る。
var emailPattern = regexp.MustCompile(
	`[A-Za-z0-9Ａ-Ｚａ-ｚ０-９.!#$%&'*+/=?^_{|}~\-．＿－` + "`" + `]+[@＠][A-Za-z0-9Ａ-Ｚａ-ｚ０-９\-－]+(?:[.．][A-Za-z0-9Ａ-Ｚａ-ｚ０-９\-－]+)*[.．][A-Za-zＡ-Ｚａ-ｚ]{2,}`,
)

// phonePattern は日本の電話番号。固定（10桁）・携帯や IP 電話（11桁）・フリーダイヤル（0120、10桁）、
// 国番号付き（+81 のあとに市外局番の 0 を除いた 9〜10桁）を拾う。
//   - 国番号のあとに 0 を残した書き方（「+81-090-…」「+81(0)90-…」）も拾う。
//     この 0 を数字として数えると桁が多すぎて、番号を丸ごと見落とすため。
//   - 数字の間に1つまで区切りを許す（ハイフンの各種・全角のハイフン・長音記号・括弧）。全角数字も拾う。
//   - 空白は区切りに含めない。「03-1234-5678 2名」の「 2」まで番号として取り込み、本文を壊すため
//     （「090 1234 5678」のような空白区切りは見落とす。+81 の直後の空白1つだけは許す）。
//   - 0 で始まる 10〜11桁に限るので、郵便番号（7桁）・金額（1,000,000 など。0 で始まらず、
//     カンマは区切りに含めない）・日付（2026-04-01）は拾わない。
//   - 誤検出として許すもの：0 で始まる 10〜11桁の番号（学籍番号・口座番号など）。これらも個人に
//     結びつく番号なので、置き換えても学習への害は小さく、見落とすより安全と判断した。
//   - より長い数字列の一部は拾わない（notInsideDigits）。
var phonePattern = regexp.MustCompile(
	`(?:[+＋][8８][1１] ?(?:[-‐‑–—−－ー]?[(（]?[0０][)）]?)?|[0０])(?:[-‐‑–—−－ー()（）]?[0-9０-９]){9,10}`,
)

// notInsideDigits は一致の直前と直後が数字でないことを見る。Go の regexp には前後読みが無いため。
// 「012345678901234」のような長い数字列の頭の 11桁を電話番号として置き換えないようにする。
func notInsideDigits(text string, start, end int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(text[:start]); unicode.IsDigit(r) {
			return false
		}
	}
	if end < len(text) {
		if r, _ := utf8.DecodeRuneInString(text[end:]); unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// addressPattern は住所。「都道府県＋市区町村」の形を必須にし、続く町名と番地があればそこまで含める。
//   - 都道府県だけ（「東京都の大学」「福岡県出身」）は拾わない。都道府県だけでは個人を絞れず、
//     本文の情報（地域の話題）を残すほうを優先した。
//   - 市区町村は漢字・カタカナ 1〜7文字＋「市・区・町・村・郡」。「横浜市中区」「〇〇郡〇〇町」のように
//     2段まで続けて拾う。ひらがなの市名（さいたま市・つくば市など）のため、ひらがなだけの 2〜5文字＋「市」なども拾う。
//     このため「埼玉県にある市」のような文は誤検出になるが、まれで、置き換えても害は小さい。
//   - 町名と番地は「漢字・カタカナ（と『が』『ヶ』）0〜8文字＋数字（漢数字・全角も）＋丁目・番地・番・号・条・ハイフン」の
//     繰り返しで拾う。数字を必須にしているので「東京都千代田区出身です」は「<ADDRESS>出身です」になり、
//     「出身」は残る。一方「港区在住3年」のように市区町村のあとに数字が来ると「在住3」まで置き換わる
//     （誤検出として許す。番地を見落とすより安全と判断した）。
//   - 見落とすもの：都道府県を省いた住所（「千代田区霞が関1-2-3」）、ひらがなを含む町名の番地の部分。
var addressPattern = regexp.MustCompile(
	`(?:北海道|東京都|京都府|大阪府|` +
		`(?:青森|岩手|宮城|秋田|山形|福島|茨城|栃木|群馬|埼玉|千葉|神奈川|新潟|富山|石川|福井|山梨|長野|岐阜|静岡|愛知|三重|` +
		`滋賀|兵庫|奈良|和歌山|鳥取|島根|岡山|広島|山口|徳島|香川|愛媛|高知|福岡|佐賀|長崎|熊本|大分|宮崎|鹿児島|沖縄)県)` +
		`(?:(?:[\p{Han}々〆ヶケァ-ヴー]{1,7}?|[ぁ-ゖ]{2,5}?)[市区町村郡]){1,2}` +
		`(?:[\p{Han}々〆ヶケァ-ヴーが]{0,8}?[0-9０-９〇一二三四五六七八九十百]+(?:丁目|番地|番|号|条|[-‐‑–—−－ー])?)*`,
)
