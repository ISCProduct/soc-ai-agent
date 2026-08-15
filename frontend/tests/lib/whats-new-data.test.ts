/**
 * @jest-environment jsdom
 */
import { hasUnreadWhatsNew, markWhatsNewAsSeen, type WhatsNewEntry } from '@/lib/whats-new-data'

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
})
