package repository

import "errors"

// ErrForbidden は他人のリソースへのアクセスを表す。
// Repository が返し、Service・Controller が errors.Is で判定する。
// 下位層(repositories)が上位層(services)を import しないよう、ここに置く。
var ErrForbidden = errors.New("forbidden")
