// Package piimask は、学生の文章を外部の LLM・API へ送る前に、個人を特定できる語を伏字にする。
//
// 学習データの書き出し用の masking パッケージを土台にし、LLM へ送る用途に合わせて項目を足している。
//
//   - 企業名は伏字にしない。応募先の企業名はレビューに必要で、公開情報でもある。
//   - 生年月日・郵便番号・URL・SNS のアカウント名を足す。
//   - 氏名・学校名・メールは、その学生の登録情報から辞書を作って照合する。
//
// 守る原則（失敗したら送らない）:
//
//	呼び出し側は Masker を作れなかったとき（ForLLM がエラーを返したとき）、本文を外へ送ってはならない。
//
// 限界: 辞書にない氏名（本文に出てくる第三者の名前など）は残る。文脈から人が分かる記述も防げない。
// そのため、送る量を必要な範囲に絞ることと、ベンダー側の取り扱いの確認を併用する。
package piimask

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"Backend/internal/services/training/masking"
)

// 置き換えの記号。masking パッケージの <NAME> <EMAIL> <PHONE> <ADDRESS> <SCHOOL> と並べて使う。
const (
	TokenBirthdate = "<BIRTHDATE>"
	TokenPostal    = "<POSTAL>"
	TokenURL       = "<URL>"
	TokenHandle    = "<HANDLE>"
	TokenID        = "<ID>"
)

const (
	keyBirthdate      = "birthdate"
	keyPostal         = "postal"
	keyURL            = "url"
	keyHandle         = "handle"
	keyPhoneSp        = "phone_spaced"
	keyAddrNoPref     = "address_no_pref"
	keySpacedDigits   = "phone_spaced_digits"
	keyDomainFragment = "email_domain_fragment"
	keyObfEmail       = "email_obfuscated"
	keyAccount        = "account"
	keyIDNumber       = "id_number"
	keyKnownEmail     = "known_email"
)

// Subject は辞書を作るための、その学生の登録情報。出力には書かない。
type Subject struct {
	Name       string
	Email      string
	SchoolName string
}

// SubjectReader は userID から Subject を取り出す。
type SubjectReader interface {
	SubjectFor(userID uint) (Subject, error)
}

// SubjectReaderFunc は関数を SubjectReader として使うための型。
type SubjectReaderFunc func(userID uint) (Subject, error)

// SubjectFor は SubjectReader を満たす。
func (f SubjectReaderFunc) SubjectFor(userID uint) (Subject, error) { return f(userID) }

// Masker は LLM へ送る前の伏字の処理。複数の goroutine から同時に Mask を呼んでよい。
type Masker struct {
	m *masking.Masker
}

// ForLLM は LLM へ送る用途の Masker を作る。Subject が空なら、規則だけで照合する（辞書は空）。
// エラーを返したときは、本文を外へ送ってはならない。
func ForLLM(subj Subject) (*Masker, error) {
	var emails []string
	if e := strings.TrimSpace(subj.Email); e != "" {
		emails = append(emails, e)
		// 「taro@example」のようにトップレベルドメインが欠けた書き方も拾うため、ローカル部も辞書に入れる
		if local, _, ok := strings.Cut(e, "@"); ok && local != "" {
			emails = append(emails, local)
		}
	}
	dict := masking.Dictionary{
		masking.KeyName:   append(masking.NameTerms(subj.Name), romajiHints(subj.Email)...),
		masking.KeySchool: {strings.TrimSpace(subj.SchoolName)},
		keyKnownEmail:     emails,
	}
	m, err := masking.New(rules(), dict)
	if err != nil {
		return nil, err
	}
	return &Masker{m: m}, nil
}

// Mask は text の個人を特定できる語を、置き換えの記号にして返す。
func (m *Masker) Mask(text string) string {
	if m == nil {
		// nil の Masker は「何も伏字にしない」ことになるので、黙って通さず落とす。
		panic("piimask: nil Masker（伏字の準備ができていないまま文章を送ろうとした）")
	}
	return m.m.Mask(text)
}

func rules() []masking.Rule {
	// DefaultRules の企業名は辞書が空なので何も置き換えない（LLM へ送る用途では企業名を残す）。
	r := masking.DefaultRules()
	return append(r,
		masking.Rule{Key: keyKnownEmail, Token: masking.TokenEmail, Method: masking.MethodDictionary},
		masking.Rule{Key: keyBirthdate, Token: TokenBirthdate, Method: masking.MethodPattern, Pattern: birthdatePattern, Accept: acceptBirthdate},
		masking.Rule{Key: keyPostal, Token: TokenPostal, Method: masking.MethodPattern, Pattern: postalPattern, Accept: notInsideDigits},
		masking.Rule{Key: keyURL, Token: TokenURL, Method: masking.MethodPattern, Pattern: urlPattern},
		masking.Rule{Key: keyHandle, Token: TokenHandle, Method: masking.MethodPattern, Pattern: handlePattern, Accept: acceptHandle},
		masking.Rule{Key: keyPhoneSp, Token: masking.TokenPhone, Method: masking.MethodPattern, Pattern: phoneSpacedPattern, Accept: notInsideDigits},
		masking.Rule{Key: keyAddrNoPref, Token: masking.TokenAddress, Method: masking.MethodPattern, Pattern: addressNoPrefPattern},
		masking.Rule{Key: keySpacedDigits, Token: masking.TokenPhone, Method: masking.MethodPattern, Pattern: spacedDigitsPhonePattern, Accept: acceptSpacedDigitsPhone},
		masking.Rule{Key: keyDomainFragment, Token: masking.TokenEmail, Method: masking.MethodPattern, Pattern: domainFragmentPattern},
		masking.Rule{Key: keyObfEmail, Token: masking.TokenEmail, Method: masking.MethodPattern, Pattern: obfuscatedEmailPattern},
		masking.Rule{Key: keyAccount, Token: TokenHandle, Method: masking.MethodPattern, Pattern: accountPattern, Accept: acceptHandle},
		masking.Rule{Key: keyIDNumber, Token: TokenID, Method: masking.MethodPattern, Pattern: idNumberPattern},
	)
}

// birthdatePattern は生年月日。次の形だけを拾う。
//   - 「生年月日」「誕生日」の見出しに続く日付（見出しと値が同じ行にあるとき）
//   - 日付の直後に「生」が付くもの（「1999年4月1日生」「平成11年4月1日生まれ」）。「生成」「生産」などの語は acceptBirthdate で除く
//   - 日付のあとの年齢の書き方（「1999年4月1日（22歳）」）、年齢だけの「満22歳」
//   - 月/日/西暦（「04/01/1999」）。西暦は生年として現実的な 1940〜2012 年だけ
//
// 卒業年月（「令和4年3月 卒業」）のように、日が無い日付や、生年月日ではない日付は拾わない。
// 学歴・職歴の年月は、レビューに必要なので残す。
var birthdatePattern = regexp.MustCompile(
	`(?:生年月日|誕生日|Birth(?:day| ?date)?)[ 　]*[:：]?[ 　]*(?:` + dateBody + `)` +
		`|(?:` + dateBody + `)[ 　]*生(?:まれ)?` +
		`|(?:` + dateBody + `)[ 　]*[（(][ 　]*(?:満[ 　]*)?[0-9０-９]{1,2}[ 　]*歳[ 　]*[）)]` +
		`|満[ 　]*[0-9０-９]{1,2}[ 　]*歳` +
		`|[0-9０-９]{1,2}/[0-9０-９]{1,2}/(?:19[4-9][0-9０-９]|200[0-9０-９]|201[0-2０-２])`,
)

// dateBody は日付の本体。西暦・和暦（漢数字も）・区切りの違い（年月日・スラッシュ・ハイフン・ドット）を拾う。
const dateBody = `(?:(?:昭和|平成|令和|西暦|[SHRshr])?[0-9０-９〇一二三四五六七八九十元]{1,4}[年/.\-．][ 　]*[0-9０-９〇一二三四五六七八九十]{1,3}[月/.\-．][ 　]*[0-9０-９〇一二三四五六七八九十]{1,3}日?)`

// postalPattern は郵便番号（〒の有無によらず、3桁-4桁）。前後が数字のときは拾わない（Accept）。
// 空白は「〒」の直後だけ許す。前の空白まで拾うと、電話番号（空白のあとの 090-1234-5678）より先に始まり、郵便番号として採られる。
var postalPattern = regexp.MustCompile(`(?:〒[ 　]?)?[0-9０-９]{3}[-‐‑–—−－ー][0-9０-９]{4}|〒[ 　]?[0-9０-９]{7}`)

// urlPattern は URL。ポートフォリオや SNS の URL は、本人にたどり着けるため伏字にする。
var urlPattern = regexp.MustCompile(
	`(?i)https?://[^\s　<>"'）)」』]+` +
		// スキームの無い書き方。SNS・開発系サービスのアカウントや、www. から始まるもの
		`|(?:\bwww\.[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+|\b(?:github|gitlab|bitbucket|twitter|x|facebook|instagram|linkedin|qiita|zenn|note|wantedly|youtube|tiktok|bsky|medium|speakerdeck|slideshare|hatenablog|hatena|pixiv|behance|dribbble|atcoder|paiza|connpass)\.(?:com|dev|jp|co\.jp|app|social|me|io|net))` +
		`(?:/[^\s　<>"'）)」』]*)?`,
)

// handlePattern は SNS のアカウント名（@ のあとに英数字と下線）。メールの「@」は、メールの規則が先に採る。
var handlePattern = regexp.MustCompile(`[@＠][A-Za-z0-9_]{3,30}`)

// phoneSpacedPattern は、ハイフンの前後に空白がある電話番号（「090 - 1234 - 5678」）。
// masking の電話の規則は、区切りを数字の間に1つしか許さないため拾えない。
var phoneSpacedPattern = regexp.MustCompile(
	`[0０][0-9０-９]{1,4}[ 　]*[-‐‑–—−－ー][ 　]*[0-9０-９]{1,4}[ 　]*[-‐‑–—−－ー][ 　]*[0-9０-９]{3,4}`,
)

// addressNoPrefPattern は、都道府県を省いた住所（「港区六本木1-2-3」「大阪市北区梅田1-1」）。
// 市区町村に加えて、番地（数字＋丁目・番・号・ハイフン）が続くものだけを拾う。数字が無い地名の話題
// （「港区の会社」）は残す。町名だけでは個人を絞れないが、番地まで書くと住所になる。
var addressNoPrefPattern = regexp.MustCompile(
	`(?:[\p{Han}々〆ヶケァ-ヴー]{1,6}[市区町村郡]){1,2}` +
		`[\p{Han}々〆ヶケァ-ヴーが]{0,8}?[0-9０-９〇一二三四五六七八九十]+(?:丁目|番地|番|号|条|[-‐‑–—−－ー][0-9０-９]+)` +
		`(?:[-‐‑–—−－ー]?[0-9０-９]+(?:丁目|番地|番|号)?)*`,
)

// spacedDigitsPhonePattern は、OCR が数字の間に空白を入れた電話番号（「0 9 0 1 2 3 4 5 6 7 8」）。
// 数字の合計が 10〜11桁のものだけ採る（acceptSpacedDigitsPhone）。短い数字の並びは拾わない。
var spacedDigitsPhonePattern = regexp.MustCompile(`[0０](?:[ 　]?[-‐‑–—−－ー]?[ 　]?[0-9０-９]){9,12}`)

func acceptSpacedDigitsPhone(text string, start, end int) bool {
	if !notInsideDigits(text, start, end) {
		return false
	}
	digits := 0
	for _, r := range text[start:end] {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits == 10 || digits == 11
}

// domainFragmentPattern は、メールが2行に分かれたときの、ドメインだけの断片（行頭の「@example.com」）。
var domainFragmentPattern = regexp.MustCompile(`^[ 　]*[@＠][A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`)

// leadingDomainPattern は行頭のドメイン（「example.com」）。直前のブロックが「@」で終わるときだけ使う。
var leadingDomainPattern = regexp.MustCompile(`^[ 　]*[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`)

// EndsWithAt は、メールの途中（「…@」）で行が終わっているかを返す。次のブロックの先頭はドメインの続き。
func EndsWithAt(text string) bool {
	t := strings.TrimRight(text, " 　")
	return strings.HasSuffix(t, "@") || strings.HasSuffix(t, "＠")
}

// MaskLeadingDomain は、行頭のドメイン（メールの続き）を伏字にする。EndsWithAt の次のブロックに使う。
func MaskLeadingDomain(text string) string {
	return leadingDomainPattern.ReplaceAllString(text, masking.TokenEmail)
}

// obfuscatedEmailPattern は、「taro (at) example.com」のようにメールの「@」を書き換えたもの。
var obfuscatedEmailPattern = regexp.MustCompile(
	`[A-Za-z0-9._%+\-]+[ 　]*[(（\[][ 　]*(?:at|AT|アット|＠)[ 　]*[)）\]][ 　]*[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`,
)

// accountPattern は、「LINE ID: taro123」「Instagram: taro_dev」のように、サービス名とアカウント名が並ぶもの。
var accountPattern = regexp.MustCompile(
	`(?:` + accountServices + `)[ 　]*(?:ID|id|Id|アカウント|ユーザー名)[ 　]*[:：]?[ 　]*@?[A-Za-z0-9_.\-]{3,40}` +
		// 「ID」などの語が無いときは、値に数字か記号（_ . -）を含むものだけ。「GitHub: Docker」のような一覧を拾わない
		// 1文字の「X」は、「Mac OS X: 10.15」のような普通の文と区別できないので、この形では使わない（「X: @id」は handlePattern が拾う）
		`|(?:` + accountServicesNoX + `)[ 　]*[:：][ 　]*@?[A-Za-z0-9_.\-]*[0-9_.\-][A-Za-z0-9_.\-]*`,
)

const accountServicesNoX = `LINE|ライン|Instagram|インスタ(?:グラム)?|Twitter|ツイッター|Facebook|フェイスブック|TikTok|Discord|Skype|スカイプ|GitHub|ギットハブ|Qiita|Zenn|Slack`

const accountServices = accountServicesNoX + `|X|エックス`

// idNumberPattern は、本人に結びつく番号の見出しと値（学籍番号・受験番号・マイナンバーなど）。
var idNumberPattern = regexp.MustCompile(
	`(?:学籍番号|学生番号|受験番号|受講番号|社員番号|会員番号|マイナンバー|個人番号|パスポート番号|運転免許証?番号|保険者番号|被保険者番号)` +
		`[ 　]*[:：]?[ 　]*[A-Za-z0-9０-９\-－ー]+(?:[ 　][0-9０-９]{3,4}){0,3}`,
)

// romajiHints は、メールのローカル部（「taro.yamada」）から、氏名のローマ字の候補を取り出す。
// 登録は漢字でも、履歴書の「Taro Yamada」のようなローマ字表記を伏字にするため。3文字以上の英字だけを使い、
// 一般的な語（info・admin など）は除く。ラテン文字は単語の途中には一致しない（masking の規則）。
func romajiHints(email string) []string {
	local, _, ok := strings.Cut(strings.TrimSpace(email), "@")
	if !ok {
		return nil
	}
	var out []string
	for _, part := range regexp.MustCompile(`[._\-+0-9]+`).Split(local, -1) {
		part = strings.ToLower(part)
		if len(part) < 3 || genericLocalPart[part] {
			continue
		}
		out = append(out, part)
	}
	return out
}

var genericLocalPart = map[string]bool{
	"info": true, "admin": true, "mail": true, "contact": true, "support": true, "test": true, "user": true,
	"office": true, "sales": true, "noreply": true, "hello": true, "gmail": true, "email": true, "me": true,
}

// acceptBirthdate は、「…日生」の直後が文字（生成・生産・生活など）のときは採らない。「…日生まれ」「…日生」で終わるものは採る。
func acceptBirthdate(text string, start, end int) bool {
	if !strings.HasSuffix(text[start:end], "生") || end >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[end:])
	return !(unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r))
}

// acceptHandle は、直前が英数字でないときだけ採る（「taro@example」の途中を拾わないため）。
func acceptHandle(text string, start, _ int) bool {
	if start == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:start])
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.')
}

// notInsideDigits は一致の前後が数字でないことを見る（Go の regexp には前後読みが無いため）。
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

// contactLabel は、連絡先・本人確認の欄の見出し。見出しのあとに、区切り（空白・コロン）か行の終わり、
// または電話の見出しなら数字が続くものだけを欄とみなす。「電話応対のアルバイト〜」のような、
// 見出しの語で始まるだけの普通の文を欄にしないため。
var contactLabel = regexp.MustCompile(
	`^[ 　]*(?:` +
		`(?:氏[ 　]*名|ふりがな|フリガナ|ﾌﾘｶﾞﾅ|生年月日|現?住[ 　]*所|帰省先|連絡先|電[ 　]*話|携帯(?:電話)?|TEL|Tel|FAX|E-?mail|Ｅ-?ｍａｉｌ|メール(?:アドレス)?|性[ 　]*別|年[ 　]*齢|写[ 　]*真)(?:[ 　]*[:：][ 　]*|[ 　]+|$)` +
		`|(?:電[ 　]*話|携帯(?:電話)?|TEL|Tel|FAX)[+＋(（0-9０-９]` +
		`)`,
)

// continuationLabel は、値が複数のブロックにまたがりやすい欄の見出し（住所・連絡先・電話・メール）。
// 氏名・年齢・性別などは、続きのブロックが無いので含めない（次の学歴の行を巻き込まないため）。
var continuationLabel = regexp.MustCompile(
	`^[ 　]*(?:現?住[ 　]*所|連絡先|帰省先|電[ 　]*話|携帯(?:電話)?|TEL|Tel|FAX|E-?mail|Ｅ-?ｍａｉｌ|メール(?:アドレス)?)(?:[ 　]*[:：][ 　]*|[ 　]+|$)`,
)

// IsContinuationLabelLine は、住所・電話・メールの欄の行かを返す。この行の次のブロックは、値の続きかもしれない。
func IsContinuationLabelLine(line string) bool {
	return continuationLabel.MatchString(strings.TrimSpace(line))
}

// 続きのブロックとみなす条件。
var (
	startsWithDateYear = regexp.MustCompile(`^[ 　]*(?:(?:19|20)[0-9０-９]{2}[年/.\-]|(?:昭和|平成|令和)[0-9０-９元])`)
	buildingWord       = regexp.MustCompile(`マンション|ビル|アパート|ハイツ|コーポ|レジデンス|ハウス|荘|号室|棟|階|丁目|番地`)
	emailLike          = regexp.MustCompile(`^[A-Za-z0-9._%+\-@＠]+$`)
	// 町名のあとの番地（「六本木1-2-3」）。年の範囲（2020-2023）を拾わないよう、各1〜3桁に限る
	houseNumber = regexp.MustCompile(`(?:^|[^0-9０-９])[0-9０-９]{1,3}[-‐‑–—−－ー][0-9０-９]{1,3}(?:[^0-9０-９]|$)`)
)

// maxContinuationRunes は、続きのブロックとみなす長さの上限。
const maxContinuationRunes = 40

// IsContactContinuation は、住所・電話・メールの続きのブロックらしいかを返す（番地・建物名・電話の続き・メールの断片）。
// 年月日で始まる行（「2020年4月 入学」）は、学歴・職歴の行なので続きとみなさない。
func IsContactContinuation(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || utf8.RuneCountInString(line) > maxContinuationRunes || startsWithDateYear.MatchString(line) {
		return false
	}
	if r, _ := utf8.DecodeRuneInString(line); unicode.IsDigit(r) {
		return true
	}
	return buildingWord.MatchString(line) || emailLike.MatchString(line) || houseNumber.MatchString(line)
}

// IsBareContactLabel は、見出しだけの行（値が次のブロックにある）かを返す。
// OCR は「氏名」と「山田 太郎」を別のブロックに分けることがある。見出しだけの行の次のブロックは値なので、
// 辞書にない書き方（ふりがな・ローマ字など）でも残さず、あわせて除く。
func IsBareContactLabel(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || utf8.RuneCountInString(line) > 12 {
		return false
	}
	loc := contactLabel.FindStringIndex(line)
	return loc != nil && strings.TrimSpace(strings.Trim(line[loc[1]:], ":：")) == ""
}

// maxContactLabelRunes は、見出しから始まる行を「連絡先の欄」とみなす長さの上限。
// 長い行は本文（志望動機など）が見出しの語で始まっただけかもしれないので、欄とはみなさない。
const maxContactLabelRunes = 80

// IsContactFieldLine は、履歴書の氏名・住所・電話・メールなどの欄の行かを返す。
// レビューの価値が無く、個人を特定する値が入るので、本文を送るときは行ごと除く。
func IsContactFieldLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" || utf8.RuneCountInString(line) > maxContactLabelRunes {
		return false
	}
	return contactLabel.MatchString(line)
}
