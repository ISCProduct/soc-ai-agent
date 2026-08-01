/**
 * @jest-environment jsdom
 */
import {
  INTERVIEW_LOBBY_DRAFT_KEY,
  clearInterviewLobbyDraft,
  loadInterviewLobbyDraft,
  parseInterviewLobbyDraft,
  resolvePositionFromDraft,
  saveInterviewLobbyDraft,
} from '@/app/interview/lobbyDraft'
import { POSITIONS } from '@/app/interview/constants'

describe('lobbyDraft', () => {
  it('有効な JSON をパースする', () => {
    const draft = parseInterviewLobbyDraft(
      JSON.stringify({
        company: { id: 3, name: '株式会社テスト', industry: 'IT' },
        positionId: POSITIONS[0].id,
        positionCategory: 'general',
        savedAt: 1,
      }),
    )
    expect(draft?.company.name).toBe('株式会社テスト')
    expect(draft?.company.id).toBe(3)
    expect(draft?.positionId).toBe(POSITIONS[0].id)
  })

  it('会社名がない JSON は null', () => {
    expect(parseInterviewLobbyDraft(JSON.stringify({ company: { id: 1 } }))).toBeNull()
    expect(parseInterviewLobbyDraft(null)).toBeNull()
    expect(parseInterviewLobbyDraft('not-json')).toBeNull()
  })

  it('sessionStorage への保存・読込・削除ができる', () => {
    const store = new Map<string, string>()
    const storage = {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, v)
      },
      removeItem: (k: string) => {
        store.delete(k)
      },
    }

    saveInterviewLobbyDraft(
      {
        company: { id: 1, name: '復元テスト社' },
        positionId: POSITIONS[0].id,
        positionCategory: 'sier',
      },
      storage,
    )
    expect(store.has(INTERVIEW_LOBBY_DRAFT_KEY)).toBe(true)

    const loaded = loadInterviewLobbyDraft(storage)
    expect(loaded?.company.name).toBe('復元テスト社')
    expect(loaded?.positionCategory).toBe('sier')

    clearInterviewLobbyDraft(storage)
    expect(loadInterviewLobbyDraft(storage)).toBeNull()
  })

  it('未知の positionId はカテゴリからフォールバックする', () => {
    const pos = resolvePositionFromDraft('unknown-id', 'general')
    expect(pos.category).toBe('general')
    expect(POSITIONS.some((p) => p.id === pos.id)).toBe(true)
  })
})
