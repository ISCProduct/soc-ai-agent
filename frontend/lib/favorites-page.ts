export const FAVORITES_PAGE_SIZE = 100

/** お気に入り一覧の取得URL。1回の件数は100のまま、開始位置だけを進める。 */
export function favoritesListUrl(offset: number): string {
  const safe = Number.isFinite(offset) && offset > 0 ? Math.floor(offset) : 0
  return `/api/chat/favorites?limit=${FAVORITES_PAGE_SIZE}&offset=${safe}`
}

/** total が無い応答は、今見えている件数を社数にする。 */
export function readFavoritesTotal(total: unknown, pageCount: number): number {
  return typeof total === 'number' && Number.isFinite(total) ? total : pageCount
}

/**
 * ハート用の企業ID。全社分が無いときは、今のページのIDに戻す。
 */
export function readFavoriteCompanyIds(ids: unknown, fallback: string[]): string[] {
  if (!Array.isArray(ids)) return fallback
  return ids.map((id) => String(id))
}
