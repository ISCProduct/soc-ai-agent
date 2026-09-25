package middleware

import (
	"context"

	"Backend/internal/models"
)

const CompanyUserIDContextKey contextKey = "companyUserID"
const CompanyIDContextKey contextKey = "companyID"

// CompanyUserRoleContextKey は company_users.role（owner / member）。
// 破壊的操作を owner に限るために使う（#1319 の共通方針）。
// 認証時に1度だけ引いた値を載せ、ハンドラごとに再取得しない。
const CompanyUserRoleContextKey contextKey = "companyUserRole"

// CompanyUserIDFromContext は企業ユーザーIDを返す。
func CompanyUserIDFromContext(ctx context.Context) (uint, bool) {
	v := ctx.Value(CompanyUserIDContextKey)
	id, ok := v.(uint)
	return id, ok && id > 0
}

// CompanyIDFromContext は企業ユーザーが所属する company_id を返す。
func CompanyIDFromContext(ctx context.Context) (uint, bool) {
	v := ctx.Value(CompanyIDContextKey)
	id, ok := v.(uint)
	return id, ok && id > 0
}

// CompanyUserRoleFromContext は企業ユーザーの役割を返す。
func CompanyUserRoleFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(CompanyUserRoleContextKey)
	role, ok := v.(string)
	return role, ok && role != ""
}

// CompanyUserIsOwner は企業ユーザーが owner かを返す。
//
// 役割が取れない場合は false を返す。取れないのは認証を通っていない
// ときだけで、その場合に owner 扱いすると権限判定が素通りする。
func CompanyUserIsOwner(ctx context.Context) bool {
	role, ok := CompanyUserRoleFromContext(ctx)
	return ok && role == models.CompanyUserRoleOwner
}
