package repository

import "errors"

// ErrDuplicateActiveApplication は、同じユーザー・同じ企業に、進行中の応募が既にあることを表す。
// 先に確認して見つけた場合と、並行リクエストの競合で DB の一意制約に弾かれた場合の両方で返す。
var ErrDuplicateActiveApplication = errors.New("duplicate_active_application")
