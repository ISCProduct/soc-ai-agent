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
)

const (
	keyBirthdate  = "birthdate"
	keyPostal     = "postal"
	keyURL        = "url"
	keyHandle     = "handle"
	keyKnownEmail = "known_email"
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
		masking.KeyName:   masking.NameTerms(subj.Name),
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
		masking.Rule{Key: keyBirthdate, Token: TokenBirthdate, Method: masking.MethodPattern, Pattern: birthdatePattern},
		masking.Rule{Key: keyPostal, Token: TokenPostal, Method: masking.MethodPattern, Pattern: postalPattern, Accept: notInsideDigits},
		masking.Rule{Key: keyURL, Token: TokenURL, Method: masking.MethodPattern, Pattern: urlPattern},
		masking.Rule{Key: keyHandle, Token: TokenHandle, Method: masking.MethodPattern, Pattern: handlePattern, Accept: acceptHandle},
	)
}

// birthdatePattern は生年月日。次のどちらかの形だけを拾う。
//   - 「生年月日」「誕生日」の見出しに続く日付（見出しと値が同じ行にあるとき）
//   - 日付の直後に「生」が付くもの（「1999年4月1日生」「平成11年4月1日生まれ」）
//   - 年齢の書き方（「満22歳」）
//
// 卒業年月（「令和4年3月 卒業」）のように、日が無い日付や、生年月日ではない日付は拾わない。
// 学歴・職歴の年月は、レビューに必要なので残す。
var birthdatePattern = regexp.MustCompile(
	`(?:生年月日|誕生日|Birth(?:day| ?date)?)[ 　]*[:：]?[ 　]*(?:(?:昭和|平成|令和|西暦)?[0-9０-９元]{1,4}[年/.\-．][ 　]*[0-9０-９]{1,2}[月/.\-．][ 　]*[0-9０-９]{1,2}日?)` +
		`|(?:(?:昭和|平成|令和|西暦)?[0-9０-９元]{1,4}[年/.\-．][ 　]*[0-9０-９]{1,2}[月/.\-．][ 　]*[0-9０-９]{1,2}日?[ 　]*生(?:まれ)?)` +
		`|満[ 　]*[0-9０-９]{1,2}[ 　]*歳`,
)

// postalPattern は郵便番号（〒の有無によらず、3桁-4桁）。前後が数字のときは拾わない（Accept）。
// 空白は「〒」の直後だけ許す。前の空白まで拾うと、電話番号（空白のあとの 090-1234-5678）より先に始まり、郵便番号として採られる。
var postalPattern = regexp.MustCompile(`(?:〒[ 　]?)?[0-9０-９]{3}[-‐‑–—−－ー][0-9０-９]{4}`)

// urlPattern は URL。ポートフォリオや SNS の URL は、本人にたどり着けるため伏字にする。
var urlPattern = regexp.MustCompile(`https?://[^\s　<>"'）)」』]+`)

// handlePattern は SNS のアカウント名（@ のあとに英数字と下線）。メールの「@」は、メールの規則が先に採る。
var handlePattern = regexp.MustCompile(`@[A-Za-z0-9_]{3,30}`)

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
		`(?:氏[ 　]*名|ふりがな|フリガナ|ﾌﾘｶﾞﾅ|生年月日|現?住[ 　]*所|連絡先|電[ 　]*話|携帯(?:電話)?|TEL|Tel|FAX|E-?mail|Ｅ-?ｍａｉｌ|メール(?:アドレス)?|性[ 　]*別|年[ 　]*齢|写[ 　]*真)(?:[ 　]*[:：][ 　]*|[ 　]+|$)` +
		`|(?:電[ 　]*話|携帯(?:電話)?|TEL|Tel|FAX)[+＋(（0-9０-９]` +
		`)`,
)

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
