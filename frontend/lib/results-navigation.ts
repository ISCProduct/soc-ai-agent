import { authService } from '@/lib/auth'

export interface ResultsSessionContext {
  userId: string
  sessionId: string
}

/** 自己分析チャットの session_id を storage から取得 */
export function getStoredChatSessionId(): string | null {
  if (typeof window === 'undefined') return null
  return (
    sessionStorage.getItem('chatSessionId') ||
    localStorage.getItem('currentSessionId') ||
    null
  )
}

/** マッチング結果ページ表示に必要な user_id / session_id を取得 */
export function getResultsSessionContext(): ResultsSessionContext | null {
  const user = authService.getStoredUser()
  const sessionId = getStoredChatSessionId()
  if (!user?.user_id || !sessionId) return null
  return {
    userId: String(user.user_id),
    sessionId,
  }
}

/** クエリ付き /results パスを生成 */
export function buildResultsPath(context: ResultsSessionContext): string {
  const params = new URLSearchParams({
    user_id: context.userId,
    session_id: context.sessionId,
  })
  return `/results?${params.toString()}`
}

/** セッションがあれば /results、なければチャット (/) へ */
export function getResultsPathOrChat(): string {
  const context = getResultsSessionContext()
  return context ? buildResultsPath(context) : '/'
}
