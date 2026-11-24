// APIプロキシのエラー本文から利用者向けの文言を取り出す（#2569 で page-content.tsx から分離）

/**
 * APIプロキシのエラーレスポンス（{ error, status, detail }想定）から
 * 日本語の短いメッセージを取り出す。生JSON/HTMLをそのまま表示しないため(#1015)。
 *
 * 422 のみ detail を優先する。RAGが利用者向けの具体的な案内（例: 「文章が長すぎて
 * 添削できませんでした。文字数を減らしてお試しください。」#1521）を detail に入れる一方、
 * api-proxy が error へ一般文「処理に失敗しました。しばらくしてから再試行してください。」を
 * 入れるため、error だけを読むと何をすれば直るのか分からなくなる。
 *
 * ただし detail は文字列とは限らない。api-proxy の getDetailText は本文に文字列 detail が
 * 無いとレスポンス本文そのものを入れるため（FastAPIのバリデーションエラーは detail が配列で、
 * しかも input にES全文が入る）、案内文の形をしているものだけを採用する。#1015 の
 * 「生JSONを画面に出さない」を壊さないための検査。
 */
const USER_FACING_DETAIL_MAX = 200

function userFacingDetail(detail: unknown): string | undefined {
  if (typeof detail !== 'string') return undefined
  const text = detail.trim()
  if (!text || text.length > USER_FACING_DETAIL_MAX) return undefined
  if (text.startsWith('{') || text.startsWith('[')) return undefined
  return text
}

export async function readApiErrorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const data: unknown = await res.json()
    const { error, detail } = (data ?? {}) as { error?: unknown; detail?: unknown }
    const safeDetail = userFacingDetail(detail)
    if (res.status === 422 && safeDetail) return safeDetail
    if (typeof error === 'string' && error) return error
    if (safeDetail) return safeDetail
  } catch { /* ignore */ }
  return fallback
}
