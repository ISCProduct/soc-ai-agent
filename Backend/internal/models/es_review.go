package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ESReview は ES添削1回分の入力と結果（#1531）。
//
// 保存するのは添削が成功したときだけ。RAG が失敗した応答まで履歴に残すと、
// 前回との差分が「失敗した回」と比較されて意味を失う。
type ESReview struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;index" json:"user_id"`
	// OrganizationID は実行時点の所属学園。教員向けの閲覧で越境させないために複製して持つ。
	OrganizationID uint `gorm:"not null" json:"organization_id"`
	// Question は利用者が入力した設問文。QuestionKey はそれを正規化したもので、
	// 同一設問の突き合わせに使う（種別だけだと「学チカ」と「ガクチカ」が別扱いになる）。
	Question     string  `gorm:"type:varchar(500);not null" json:"question"`
	QuestionKey  string  `gorm:"type:varchar(255);not null" json:"-"`
	QuestionType string  `gorm:"type:varchar(50);not null" json:"question_type"`
	CompanyName  *string `gorm:"type:varchar(255)" json:"company_name,omitempty"`
	ESText       string  `gorm:"type:text;not null" json:"es_text"`
	// ESTextHash は正規化した本文の SHA-256。同じ本文を二度スコアに乗せないための鍵。
	ESTextHash string `gorm:"type:char(64);not null" json:"-"`
	// ScoredAt はスコアへ反映した時刻。反映していなければ nil。
	ScoredAt  *time.Time `json:"-"`
	CharLimit *uint      `json:"char_limit,omitempty"`
	// CharLimitMode は "within"（以内）か "around"（程度）。未指定なら nil。
	CharLimitMode *string `gorm:"type:varchar(20)" json:"char_limit_mode,omitempty"`

	SpecificityScore   uint8 `gorm:"not null" json:"specificity_score"`
	StarScore          uint8 `gorm:"not null" json:"star_score"`
	LengthBalanceScore uint8 `gorm:"not null" json:"length_balance_score"`
	// CompanyFitScore は企業名を指定した添削でだけ付く。0 を入れないこと（0点と未評価は別物）。
	CompanyFitScore *uint8 `json:"company_fit_score,omitempty"`

	Feedback     string    `gorm:"type:text;not null" json:"feedback"`
	ImprovedText string    `gorm:"type:text;not null" json:"improved_text"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (ESReview) TableName() string { return "es_reviews" }

// HashESText は本文を突き合わせるための鍵を返す。
//
// 「見た目が同じ本文」を同じ鍵にする。ここを緩くすると、不可視文字を1つ足す・
// 全角半角を変える・NFD にするだけで別の本文として何度でもスコアを積めてしまう
// （いずれもテストで実際に再現した経路）。
//
// ただし正規化をどれだけ強めても、一語でも書き換えられれば別の鍵になる。
// 内容の判定だけでは原理的に塞がらないので、反映の頻度そのものにも上限を置く
// （esreview.Service の scoreInterval）。ここはあくまで「同じ文章の出し直し」を
// 弾くための層。
func HashESText(text string) string {
	// NFKC: 全角半角・合成文字の違いを畳む。
	normalized := norm.NFKC.String(text)
	var b strings.Builder
	for _, r := range normalized {
		// Cf（書式用の不可視文字: ZWSP・BOM・soft hyphen・word joiner など）を落とす。
		if unicode.Is(unicode.Cf, r) || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// NormalizeESQuestion は設問文を突き合わせ用のキーに正規化する。
//
// 利用者は同じ設問を毎回まったく同じ文字列では打たない。空白の有無・全角半角・
// 末尾の句読点程度の揺れで「別の設問」になると、前回との差分が永久に出ない。
// 一方で正規化しすぎると別の設問が同一視されるため、落とすのは空白と記号だけに留める。
//
// 列は varchar(255) なので、長い設問は正規化後に切り詰める。
// 先頭255文字が一致する設問はまず同じものとみなしてよい。
func NormalizeESQuestion(question string) string {
	var b strings.Builder
	// 全角半角の揺れ（ＰＲ/PR、？/?）を先に畳む。HashESText と同じ NFKC。
	for _, r := range norm.NFKC.String(question) {
		if unicode.IsSpace(r) {
			continue
		}
		// 句読点・記号の揺れ（「？」「?」「。」の有無）は同一設問とみなす。
		// ただし + と # は残す。落とすと「C++言語」「C#」「C言語」が同じキーになり、
		// 別の設問の「前回」として差分が出てしまう。
		if (unicode.IsPunct(r) || unicode.IsSymbol(r)) && r != '+' && r != '#' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	key := b.String()
	// varchar(255) は文字数ではなくバイト数ではないが、GORM/MySQL は文字数で数える。
	// ルーン単位で切らないと多バイト文字の途中で切れる。
	runes := []rune(key)
	if len(runes) > 255 {
		return string(runes[:255])
	}
	return key
}
