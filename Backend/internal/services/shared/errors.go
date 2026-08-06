package shared

import "errors"

// ErrForbidden は複数のサービスクラスタ・コントローラで共有される汎用の権限エラー。
var ErrForbidden = errors.New("forbidden")
