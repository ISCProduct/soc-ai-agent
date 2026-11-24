/**
 * @jest-environment jsdom
 */
import { StrictMode } from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import { loadNextActions, NextActionsBar } from '@/components/home/NextActionsBar'

// SOCAIAGENT-614: ホームの「次にやること」に、診断スコアに応じた改善の提案を出す。

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }) },
}))

function mockFetch(portfolio: [number, unknown]) {
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    const u = String(url)
    if (u.startsWith('/api/schedule')) return { ok: true, status: 200, json: async () => [] }
    if (u.startsWith('/api/applications')) return { ok: true, status: 200, json: async () => ({ applications: [] }) }
    return { ok: portfolio[0] < 400, status: portfolio[0], json: async () => portfolio[1] }
  }) as unknown as typeof fetch
}

const DIAGNOSIS = {
  scores: [
    { category: '技術志向', score: 70 },
    { category: 'チームワーク志向', score: 65 },
    { category: 'リーダーシップ志向', score: 60 },
    { category: 'コミュニケーション力', score: 35 },
  ],
}

test('予定が無ければ、いちばん低い傾向の練習を先頭に出し、練習の画面へつなぐ', async () => {
  mockFetch([200, { generated_at: '', sections: ['diagnosis', 'interview'], diagnosis: DIAGNOSIS, interviews: [{ session_id: 1 }] }])
  render(<NextActionsBar />)
  expect(await screen.findByText(/「コミュニケーション力」が 35 と、ほかより低めです/)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '面接で練習する' })).toHaveAttribute('href', '/interview')
})

test('診断がまだなら勧めるが、ホームのチャットで進めるのでボタンは出さない', async () => {
  mockFetch([200, { generated_at: '', sections: ['diagnosis', 'interview'], diagnosis: { scores: [] }, interviews: [] }])
  render(<NextActionsBar />)
  expect(await screen.findByText(/AI 診断がまだです/)).toBeInTheDocument()
  expect(screen.getByText('（ほか1件）')).toBeInTheDocument()
  expect(screen.queryByRole('link')).not.toBeInTheDocument()
})

test('診断・面接の状況が取れなくても、予定と選考状況の表示は止めない', async () => {
  mockFetch([500, {}])
  render(<NextActionsBar />)
  expect(await screen.findByText(/直近2週間の予定はありません/)).toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('右パネル向けに状況を渡さず呼んだときは、改善の提案を出さず、ポートフォリオも取りに行かない', async () => {
  mockFetch([200, { generated_at: '', sections: ['diagnosis', 'interview'], diagnosis: { scores: [] }, interviews: [] }])
  await expect(loadNextActions()).resolves.toEqual([])
  const urls = (global.fetch as jest.Mock).mock.calls.map((c) => String(c[0]))
  expect(urls.some((u) => u.startsWith('/api/portfolio'))).toBe(false)
})

test('面接のレポートが無いときは、練習していないと言い切らない', async () => {
  mockFetch([200, { generated_at: '', sections: ['diagnosis', 'interview'], diagnosis: DIAGNOSIS, interviews: [] }])
  const actions = await loadNextActions(Promise.resolve({ diagnosis: DIAGNOSIS.scores, interviewCount: 0 }))
  expect(actions.map((a) => a.text)).toContain('AI 面接の練習レポートがまだありません。本番の前に一度試してみましょう')
})

test('読み込みが重なったときは、先の遅い応答で新しい結果を上書きしない（StrictMode の二重実行など）', async () => {
  const releases: Array<() => void> = []
  let scheduleCalls = 0
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    const u = String(url)
    if (u.startsWith('/api/schedule')) {
      scheduleCalls++
      const events = scheduleCalls === 1
        ? [{ company_name: '古い社', stage: '面接', title: '', scheduled_at: new Date(Date.now() + 86_400_000).toISOString() }]
        : []
      return new Promise((resolve) => {
        releases.push(() => resolve({ ok: true, status: 200, json: async () => events }))
      })
    }
    if (u.startsWith('/api/applications')) return { ok: true, status: 200, json: async () => ({ applications: [] }) }
    return { ok: false, status: 500, json: async () => ({}) }
  }) as unknown as typeof fetch

  render(<StrictMode><NextActionsBar /></StrictMode>)
  await waitFor(() => expect(releases).toHaveLength(2))
  // 新しい方が先に返り、古い方が後から返る
  await act(async () => { releases[1]() })
  expect(await screen.findByText(/直近2週間の予定はありません/)).toBeInTheDocument()
  await act(async () => { releases[0]() })
  expect(screen.queryByText(/古い社/)).not.toBeInTheDocument()
  expect(screen.getByText(/直近2週間の予定はありません/)).toBeInTheDocument()
})
