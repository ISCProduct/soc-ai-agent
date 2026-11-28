/**
 * @jest-environment ./tests/env/jsdom-asia-tokyo.ts
 */
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import PageContent from '@/app/schedule/page-content'

// #2092: 選考スケジュールの画面の回帰テスト（カレンダーの描画・保存・日本時間での日付の表示）。
// 日時は実行環境のタイムゾーンで扱われる（利用者の端末＝日本時間）。テストも日本時間に固定し
// （先頭の @jest-environment。CI の runner は UTC のため、テストの中で process.env.TZ を変えても効かない）、
// UTC と日本時間で日付がずれる時刻（UTC 15:30 ＝ 日本時間の翌日 0:30。#137 で直したずれ）を確かめる。

const mockPush = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, back: jest.fn(), replace: jest.fn() }),
}))

let mockStoredUser: { user_id: number } | null = { user_id: 1 }
jest.mock('@/lib/auth', () => ({
  authService: { getStoredUser: () => mockStoredUser },
}))

type Ev = { id: number; user_id: number; company_name: string; stage: string; title: string; scheduled_at: string; notes: string }

/** 今月の day 日（日本時間）の前日 15:30 UTC ＝ day 日 0:30（日本時間）の ISO */
function isoAtJstMidnight30(day: number): string {
  const now = new Date()
  return new Date(Date.UTC(now.getFullYear(), now.getMonth(), day - 1, 15, 30)).toISOString()
}

function mockFetch(events: Ev[], options: { getStatus?: number } = {}) {
  const calls: Array<{ url: string; method: string; body?: unknown }> = []
  global.fetch = jest.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: init?.body ? JSON.parse(String(init.body)) : undefined })
    if (method === 'GET') {
      const status = options.getStatus ?? 200
      return { ok: status < 400, status, json: async () => events, text: async () => '' } as Response
    }
    return { ok: true, status: 200, json: async () => ({}), text: async () => '' } as Response
  }) as unknown as typeof fetch
  return calls
}

const event = (over: Partial<Ev> = {}): Ev => ({
  id: 7,
  user_id: 1,
  company_name: 'サンプル株式会社',
  stage: '1次面接',
  title: '',
  scheduled_at: isoAtJstMidnight30(15),
  notes: '',
  ...over,
})

describe('選考スケジュールの画面', () => {
  beforeEach(() => {
    mockStoredUser = { user_id: 1 }
    mockPush.mockClear()
  })

  it('未ログインならログイン画面へ送る', () => {
    mockStoredUser = null
    mockFetch([])
    render(<PageContent />)
    expect(mockPush).toHaveBeenCalledWith('/login')
  })

  it('今月のカレンダー（年月・曜日の見出し）を出す', async () => {
    mockFetch([])
    render(<PageContent />)
    const now = new Date()
    expect(screen.getByText(`${now.getFullYear()}年 ${now.getMonth() + 1}月`)).toBeInTheDocument()
    for (const d of ['日', '月', '火', '水', '木', '金', '土']) expect(screen.getAllByText(d).length).toBeGreaterThan(0)
    expect(await screen.findByText('この月の予定はまだありません')).toBeInTheDocument()
  })

  it('UTC では前日になる時刻の予定も、一覧には日本時間の日付と時刻で出す（#137）', async () => {
    mockFetch([event()])
    render(<PageContent />)
    const now = new Date()
    const listItem = (await screen.findAllByText('サンプル株式会社')).at(-1) as HTMLElement
    const card = listItem.closest('.MuiPaper-root') as HTMLElement
    expect(within(card).getByText(new RegExp(`${now.getMonth() + 1}月15日`))).toBeInTheDocument()
    expect(within(card).getByText(/00:30/)).toBeInTheDocument()
  })

  it('カレンダーでは、日本時間の日付（15日）のマスに出し、UTC の日付（14日）のマスには出さない', async () => {
    mockFetch([event()])
    const { container } = render(<PageContent />)
    await screen.findAllByText('サンプル株式会社')
    // マス＝最初の子が日付の数字で、子が2つ（数字と予定の入れ物）の div
    const cellOf = (day: number) =>
      Array.from(container.querySelectorAll('div')).find(
        (el) => el.firstElementChild?.textContent === String(day) && el.children.length === 2,
      ) as HTMLElement | undefined
    const cell15 = cellOf(15)
    const cell14 = cellOf(14)
    // 探し方がマスの DOM の形に頼るので、形が変わって見つからないまま「14日に無い」が通らないよう、両方あることを先に確かめる
    expect(cell15).toBeDefined()
    expect(cell14).toBeDefined()
    expect(cell15?.textContent).toContain('サンプル株式会社')
    expect(cell14?.textContent).not.toContain('サンプル株式会社')
  })

  it('月を移動できる（次の月・前の月）', async () => {
    mockFetch([])
    render(<PageContent />)
    const now = new Date()
    const next = new Date(now.getFullYear(), now.getMonth() + 1, 1)
    fireEvent.click(screen.getByRole('button', { name: '次の月' }))
    expect(screen.getByText(`${next.getFullYear()}年 ${next.getMonth() + 1}月`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '前の月' }))
    expect(screen.getByText(`${now.getFullYear()}年 ${now.getMonth() + 1}月`)).toBeInTheDocument()
  })

  it('予定を追加すると、入力した日本時間を UTC の ISO にして POST し、一覧を取り直す', async () => {
    const calls = mockFetch([])
    render(<PageContent />)
    await screen.findByText('この月の予定はまだありません')
    fireEvent.click(screen.getByRole('button', { name: '追加' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('イベントを追加')).toBeInTheDocument()
    fireEvent.change(within(dialog).getByLabelText(/企業名/), { target: { value: 'テスト商事' } })
    fireEvent.change(within(dialog).getByLabelText(/^日時/), { target: { value: '2026-12-01T10:00' } })
    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }))

    await waitFor(() => expect(calls.some((c) => c.method === 'POST')).toBe(true))
    const post = calls.find((c) => c.method === 'POST')!
    expect(post.url).toBe('/api/schedule')
    expect(post.body).toMatchObject({ company_name: 'テスト商事', scheduled_at: '2026-12-01T01:00:00.000Z' })
    await waitFor(() => expect(calls.filter((c) => c.method === 'GET').length).toBe(2))
  })

  it('予定を編集すると、日時を日本時間で出し、PUT で保存する', async () => {
    const calls = mockFetch([event()])
    render(<PageContent />)
    await screen.findAllByText('サンプル株式会社')
    fireEvent.click(screen.getByRole('button', { name: '予定を編集' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('イベントを編集')).toBeInTheDocument()
    const now = new Date()
    const mm = String(now.getMonth() + 1).padStart(2, '0')
    expect(within(dialog).getByLabelText(/^日時/)).toHaveValue(`${now.getFullYear()}-${mm}-15T00:30`)

    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
    const put = calls.find((c) => c.method === 'PUT')!
    expect(put.url).toBe('/api/schedule/7')
    expect(put.body).toMatchObject({ scheduled_at: isoAtJstMidnight30(15) })
  })

  it('一覧を取れなかったら、エラーを出す', async () => {
    mockFetch([], { getStatus: 500 })
    render(<PageContent />)
    expect(await screen.findByText('スケジュールの取得に失敗しました')).toBeInTheDocument()
  })
})
