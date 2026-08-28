package middleware

import "context"

const CompanyUserIDContextKey contextKey = "companyUserID"
const CompanyIDContextKey contextKey = "companyID"

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
