package repository

import "gorm.io/gorm"

// ErrNotFound は「該当する行が無い」ことを表す。Repository が返し、
// Service・Controller が errors.Is で判定する。
//
// 値は gorm.ErrRecordNotFound と同一なので、Repository の実装(GORM の First など)は
// 変換せずにそのまま返せる。Service・Controller が gorm を import せずに判定できるよう、
// ここに集約している。ORM を替えるときは、この1行だけを直せばよい。
var ErrNotFound = gorm.ErrRecordNotFound
