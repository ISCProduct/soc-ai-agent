/**
 * @jest-environment jsdom
 */
import { hasUnreadWhatsNew, markWhatsNewAsSeen, fetchWhatsNewEntries, type WhatsNewEntry } from '@/lib/whats-new-data'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ 'X-User-Token': 'user-jwt' }),
  },
}))

describe('whats-new-data', () => {
  const entries: WhatsNewEntry[] = [
    { title: '新機能A', summary: '説明A', merged_at: '2026-08-15T00:00:00Z' },
    { title: '過去の更新', summary: '説明B', merged_at: '2026-08-01T00:00:00Z' },
  ]

  beforeEach(() => {
    window.localStorage.clear()
  })

  it('未読なし(空配列)ではバナーを出さない', () => {
    expect(hasUnreadWhatsNew([])).toBe(false)
  })

  it('既読情報がなければ未読扱いにする', () => {
    expect(hasUnreadWhatsNew(entries)).toBe(true)
  })

  it('最新エントリを既読にすると未読ではなくなる', () => {
    markWhatsNewAsSeen(entries)
    expect(hasUnreadWhatsNew(entries)).toBe(false)
  })

  it('さらに新しいエントリが追加されると再び未読になる', () => {
    markWhatsNewAsSeen(entries)
    const withNew: WhatsNewEntry[] = [
      { title: '新機能B', summary: '説明C', merged_at: '2026-08-20T00:00:00Z' },
      ...entries,
    ]
    expect(hasUnreadWhatsNew(withNew)).toBe(true)
  })

  it('取得時にユーザー認証ヘッダーを付与する', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => [] })
    await fetchWhatsNewEntries()
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/whats-new',
      expect.objectContaining({
        cache: 'no-store',
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })
})
