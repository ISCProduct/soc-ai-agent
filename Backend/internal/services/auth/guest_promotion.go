package auth

import (
	"Backend/domain/entity"
	"errors"
	"fmt"
	"strings"
)

// ErrGuestNotPromotable は昇格対象として受け付けられないゲストを示す。
//
// 呼び出し側はこれを「引き継ぎせず通常登録する」合図として扱ってはいけない。
// 黙って新規作成すると、学生は診断をやり直すことになり、しかもそれが
// 画面上は成功に見える（#1374）。
var ErrGuestNotPromotable = errors.New("guest account cannot be promoted")

// PromotableGuest は昇格できるゲストかを判定する純関数。
//
// ゲストのまま診断まで進める仕様なので、本登録時にアカウントを作り直すと
// user_id が変わり、診断結果・マッチ結果・チャット履歴が取り残される。
// 教員向けの一覧は is_guest=false の生徒しか見ないため、取り残された結果は
// 誰にも届かない（#1374）。
//
// 同じ行を昇格させれば user_id が変わらず、引き継ぎ漏れが構造的に起きない。
// 移送方式だと user_id を参照する表を列挙する必要があり、1つ落とすだけで
// 静かにデータが欠ける。
func PromotableGuest(u *entity.User) error {
	if u == nil {
		return ErrGuestNotPromotable
	}
	// 既に本登録済みのアカウントを上書きすると、他人のアカウントを
	// 乗っ取れてしまう。ゲストに限る。
	if !u.IsGuest {
		return ErrGuestNotPromotable
	}
	if u.IsAdmin {
		return ErrGuestNotPromotable
	}
	if u.WithdrawnAt != nil {
		return ErrGuestNotPromotable
	}
	return nil
}

// applyRegistrationToGuest はゲスト行を本登録済みの内容で上書きする。
//
// user_id・organization_id・作成日時は変えない。診断結果やチャット履歴が
// これらに紐づいているため、変えると引き継ぎの意味が無くなる。
// schoolName は「リクエストで実際に指定された学校名」。既定値で補完したものを
// 渡してはいけない（ゲストが設定済みの学校名を既定値で潰す）。
func applyRegistrationToGuest(u *entity.User, req RegisterRequest, schoolName, hashedPassword string, schoolID *uint) {
	u.Email = req.Email
	u.Password = hashedPassword
	u.Name = req.Name
	u.IsGuest = false
	u.TargetLevel = req.TargetLevel
	u.CertificationsAcquired = req.CertificationsAcquired
	u.CertificationsInProgress = req.CertificationsInProgress

	// 学校名は入力があるときだけ上書きする。ゲストには既定の学校名が
	// 入っており、入力が空のときにそれを消すと学校の紐付けを失う。
	if strings.TrimSpace(schoolName) != "" {
		u.SchoolName = schoolName
	}
	// school_id は解決できたときだけ更新する。解決できない名前で
	// 既存の紐付けを消すと、担当校の教員から生徒が見えなくなる。
	if schoolID != nil {
		u.SchoolID = schoolID
	}
}

// guestPromotionLogLabel は昇格時のログ表記。メールアドレスは出さない。
func guestPromotionLogLabel(userID uint) string {
	return fmt.Sprintf("user_id=%d", userID)
}
