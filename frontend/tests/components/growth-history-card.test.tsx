/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { GrowthHistoryCard } from '@/components/profile/GrowthHistoryCard'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ Authorization: 'Bearer test' }),
  },
}))

const originalFetch = global.fetch

function mockFetch(body: unknown, ok = true) {
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    expect(String(input)).toContain('/api/user/growth-history')
    expect(String(input)).not.toContain('user_id=')
    return {
      ok,
      status: ok ? 200 : 500,
      json: async () => body,
    } as Response
  })
}

afterEach(() => {
  global.fetch = originalFetch
})

describe('GrowthHistoryCard', () => {
  it('前回より伸びたカテゴリと、進んだ診断を言葉で出す', async () => {
    mockFetch({
      points: [{ id: 1 }, { id: 2 }],
      has_previous: true,
      categories: [
        { category: '技術志向', previous: 50, current: 70, delta: 20, change_label: '伸びた' },
        { category: '安定志向', previous: 60, current: 60, delta: 0, change_label: '変化なし' },
      ],
      phase: { change_label: '進んだ', newly_completed: ['興味'] },
    })
    render(<GrowthHistoryCard />)
    expect(await screen.findByText('技術志向：伸びた（+20）')).toBeInTheDocument()
    expect(screen.getByText('安定志向：変化なし')).toBeInTheDocument()
    expect(screen.getByText(/診断の進み：進んだ/)).toBeInTheDocument()
    expect(screen.getByText(/新しく終わった段階：興味/)).toBeInTheDocument()
    expect(screen.getByText(/確定した適性ではありません/)).toBeInTheDocument()
  })

  it('記録が1件のときは、まだ比べられないと出す', async () => {
    mockFetch({
      points: [{ id: 1 }],
      has_previous: false,
      categories: [],
      phase: { change_label: '比較できない', newly_completed: [] },
    })
    render(<GrowthHistoryCard />)
    expect(await screen.findByText(/まだ比べられる記録がありません/)).toBeInTheDocument()
  })

  it('読み込みに失敗したら再読み込みできる', async () => {
    mockFetch({}, false)
    render(<GrowthHistoryCard />)
    expect(await screen.findByText('成長の記録を読み込めませんでした')).toBeInTheDocument()
    mockFetch({
      points: [],
      has_previous: false,
      categories: [],
      phase: { change_label: '比較できない', newly_completed: [] },
    })
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(await screen.findByText(/まだ記録がありません/)).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByText('成長の記録を読み込めませんでした')).not.toBeInTheDocument()
    })
  })
})
