/** @jest-environment jsdom */
import { render, screen, waitFor } from '@testing-library/react'
import { AIBudgetNotice } from '@/components/home/AIBudgetNotice'

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({}) },
}))

const period = (limit: number, remaining = limit) => ({ limit_usd: limit, remaining_usd: remaining })

// waitFor 自身もタイマーを使うので、60秒間隔のポーリングのタイマーだけを取り出す。
function pollTimerId(spy: jest.SpyInstance): unknown {
  const index = spy.mock.calls.findIndex((call) => call[1] === 60_000)
  return index >= 0 ? spy.mock.results[index].value : undefined
}

function mockBudget(body: unknown, ok = true) {
  global.fetch = jest.fn().mockResolvedValue({ ok, json: async () => body }) as unknown as typeof fetch
}

describe('AIBudgetNotice', () => {
  it('上限がない利用者には何も表示せず、問い合わせを止める', async () => {
    const setIntervalSpy = jest.spyOn(window, 'setInterval')
    const clearIntervalSpy = jest.spyOn(window, 'clearInterval')
    mockBudget({ user: { daily: period(0), monthly: period(0) }, school_blocked: false, limit_configured: false, allowed: true })
    const { container } = render(<AIBudgetNotice />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(1))
    const timer = pollTimerId(setIntervalSpy)
    expect(timer).toBeDefined()
    await waitFor(() => expect(clearIntervalSpy).toHaveBeenCalledWith(timer))
    expect(container).toBeEmptyDOMElement()
    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('取得に失敗してもエラーを表示せず、回復を待って問い合わせを続ける', async () => {
    const setIntervalSpy = jest.spyOn(window, 'setInterval')
    const clearIntervalSpy = jest.spyOn(window, 'clearInterval')
    mockBudget({ error: 'failed' }, false)
    const { container } = render(<AIBudgetNotice />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(container).toBeEmptyDOMElement()
    const timer = pollTimerId(setIntervalSpy)
    expect(timer).toBeDefined()
    expect(clearIntervalSpy).not.toHaveBeenCalledWith(timer)
    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('学校の上限だけがある利用者は問い合わせを続ける', async () => {
    const setIntervalSpy = jest.spyOn(window, 'setInterval')
    const clearIntervalSpy = jest.spyOn(window, 'clearInterval')
    mockBudget({ user: { daily: period(0), monthly: period(0) }, school_blocked: false, limit_configured: true, allowed: true })
    const { container } = render(<AIBudgetNotice />)
    await waitFor(() => expect(global.fetch).toHaveBeenCalled())
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(container).toBeEmptyDOMElement()
    const timer = pollTimerId(setIntervalSpy)
    expect(timer).toBeDefined()
    expect(clearIntervalSpy).not.toHaveBeenCalledWith(timer)
    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('個人の上限があれば残りを表示する', async () => {
    mockBudget({ user: { daily: period(1, 0.25), monthly: period(0) }, school_blocked: false, limit_configured: true, allowed: true })
    render(<AIBudgetNotice />)
    expect(await screen.findByText(/今日はあと \$0\.25/)).toBeInTheDocument()
  })

  it('学校の上限で止まっているときは金額を出さずに伝える', async () => {
    mockBudget({ user: { daily: period(0), monthly: period(0) }, school_blocked: true, limit_configured: true, allowed: false })
    render(<AIBudgetNotice />)
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('学校のAI利用上限')
    expect(alert).not.toHaveTextContent('$')
  })
})
