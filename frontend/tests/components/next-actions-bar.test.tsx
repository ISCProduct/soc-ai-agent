/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { NextActionsBar } from '@/components/home/NextActionsBar'

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
