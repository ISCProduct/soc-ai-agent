// Backend の defaultAdminTokenTTL（Backend/internal/middleware/admin_token.go）と同じ7日。
// ADMIN_TOKEN_TTL_HOURS でサーバ側だけ短くすると、ここは追従せず期限切れを新しいと誤判定して
// 管理画面が 403 になる。短縮するときはこの定数も同時に変更すること（Backend/.env.example 参照）。
export const ADMIN_TOKEN_TTL_SECONDS = 7 * 24 * 60 * 60

// 期限の1時間前から取り直す。ログインセッション（1時間）の更新より長く放置しない。
export const ADMIN_TOKEN_REFRESH_MARGIN_SECONDS = 60 * 60

/** 発行時刻（unix秒）。形式は "<発行時刻>.<HMAC>"。読めなければ null。 */
export function adminTokenIssuedAt(token: string): number | null {
  const raw = token.split('.')[0]
  if (!raw || !/^\d+$/.test(raw)) return null
  const issued = Number(raw)
  if (!Number.isSafeInteger(issued)) return null
  return issued
}

/** 無い・壊れている・期限が近い管理者トークンは取り直す。 */
export function adminTokenNeedsRefresh(
  token: string | null | undefined,
  nowSeconds = Date.now() / 1000,
): boolean {
  if (!token) return true
  const issued = adminTokenIssuedAt(token)
  if (issued === null) return true
  const remaining = issued + ADMIN_TOKEN_TTL_SECONDS - nowSeconds
  return remaining < ADMIN_TOKEN_REFRESH_MARGIN_SECONDS
}
