import { parseChatSessionsResponse } from '@/app/chat-history/parseSessions'

describe('parseChatSessionsResponse', () => {
  const sample = {
    session_id: 'abc-123',
    user_id: 1,
    started_at: '2026-01-01T00:00:00Z',
    last_message_at: '2026-01-01T01:00:00Z',
    message_count: 3,
  }

  it('配列レスポンスをそのまま返す', () => {
    expect(parseChatSessionsResponse([sample])).toEqual([sample])
  })

  it('{ data: [...] } 形式を展開する', () => {
    expect(parseChatSessionsResponse({ data: [sample] })).toEqual([sample])
  })

  it('不正な形式は空配列を返す', () => {
    expect(parseChatSessionsResponse(null)).toEqual([])
    expect(parseChatSessionsResponse({})).toEqual([])
    expect(parseChatSessionsResponse({ data: 'x' })).toEqual([])
  })
})
