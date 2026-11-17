package repository

import "errors"

// ErrCompanyUserEmailTaken は、企業ユーザーのメールアドレスが、既に登録されていることを表す。
// 事前の検索と INSERT の間に同じアドレスが登録された場合(DB の一意制約に弾かれた場合)に返す。
var ErrCompanyUserEmailTaken = errors.New("company user email already registered")
