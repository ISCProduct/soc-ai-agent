/** @jest-environment jsdom */
import { render, screen, waitFor } from '@testing-library/react'
import { AIBudgetNotice } from '@/components/home/AIBudgetNotice'

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({}) },
}))

const period = (limit: number, remaining = limit) => ({ limit_usd: limit, remaining_usd: remaining })

function mockBudget(body: unknown, ok = true) {
  global.fetch = jest.fn().mockResolvedValue({ ok, json: async () => body }) as unknown as typeof fetch
}

describe('AIBudgetNotice', () => {
  it('上限がない利用者には何も表示せず、問い合わせを続けない', async () => {
    const setIntervalSpy = jest.spyOn(window, 'setInterval')
    mockBudget({ user: { daily: period(0), monthly: period(0) }, school_blocked: false, allowed: true })
    const { container } = render(<AIBudgetNotice />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(container).toBeEmptyDOMElement()
    // waitFor 自身もタイマーを使うので、ポーリングの間隔で絞る。
    expect(setIntervalSpy).not.toHaveBeenCalledWith(expect.anything(), 60_000)
    setIntervalSpy.mockRestore()
  })

  it('取得に失敗してもエラーを表示しない', async () => {
    mockBudget({ error: 'failed' }, false)
    const { container } = render(<AIBudgetNotice />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('個人の上限があれば残りを表示する', async () => {
    mockBudget({ user: { daily: period(1, 0.25), monthly: period(0) }, school_blocked: false, allowed: true })
    render(<AIBudgetNotice />)
    expect(await screen.findByText(/今日はあと \$0\.25/)).toBeInTheDocument()
  })

  it('学校の上限で止まっているときは金額を出さずに伝える', async () => {
    mockBudget({ user: { daily: period(0), monthly: period(0) }, school_blocked: true, allowed: false })
    render(<AIBudgetNotice />)
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('学校のAI利用上限')
    expect(alert).not.toHaveTextContent('$')
  })
})
