export type ChatSessionSummary = {
  session_id: string
  user_id: number
  started_at: string
  last_message_at: string
  message_count: number
}

/**
 * チャット履歴 API レスポンスを正規化する。
 * buildProxyJsonResponse は配列を { data: [...] } にラップするため両形式に対応する。
 */
export function parseChatSessionsResponse(raw: unknown): ChatSessionSummary[] {
  if (Array.isArray(raw)) {
    return raw as ChatSessionSummary[]
  }
  if (raw && typeof raw === 'object' && Array.isArray((raw as { data?: unknown }).data)) {
    return (raw as { data: ChatSessionSummary[] }).data
  }
  return []
}
