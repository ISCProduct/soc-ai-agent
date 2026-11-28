/**
 * @jest-environment jsdom
 */
import { act, render, screen, fireEvent } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page-content'

// SOCAIAGENT-714 (#2573) の不具合を固定する
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSearchParams: () => new URLSearchParams(),
}))

const ES_PLACEHOLDER = /チームで開発した経験があります/

const REWRITE_RESULT = {
  rewritten_text: '書き直した合成の文章です。',
  star: { situation: '状況の例', task: '課題の例', action: '施策の例', result: '成果の例' },
}

const submitRewrite = (text: string) => {
  fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: text } })
  fireEvent.click(screen.getByRole('button', { name: '書き直す' }))
}

describe('ESRewritePage 不具合修正 (#2573)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
    jest.useRealTimers()
  })

  it('送信中はモードを切り替えられず、返ってきた結果は送信したモードに出る', async () => {
    let resolveFetch: (value: unknown) => void = () => {}
    ;(global.fetch as jest.Mock).mockReturnValue(new Promise(resolve => { resolveFetch = resolve }))
    render(<ESRewritePage />)

    submitRewrite('元の合成文章です。')
    expect(screen.getByRole('button', { name: 'ES添削' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'ESリライト（STAR法）' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    await act(async () => {
      resolveFetch({ ok: true, json: async () => REWRITE_RESULT })
    })

    expect(await screen.findByText('リライト後')).toBeInTheDocument()
    expect(screen.getByText('ESリライト — 元の文章を入力')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'ES添削' })).toBeEnabled()
  })

  it('リライト結果の「元の文章」は送信時の文章のままで、入力欄の編集に連動しない', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REWRITE_RESULT })
    render(<ESRewritePage />)

    submitRewrite('送信した合成文章です。')
    await screen.findByText('リライト後')

    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '送信後に編集した合成文章です。' } })
    expect(screen.getByText('送信した合成文章です。')).toBeInTheDocument()
  })

  it('コピーに失敗したらエラーで知らせ、「コピーしました」にしない', async () => {
    const writeText = jest.fn().mockRejectedValue(new Error('denied'))
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REWRITE_RESULT })
    render(<ESRewritePage />)

    submitRewrite('元の合成文章です。')
    await screen.findByText('リライト後')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'リライト結果をコピー' }))
    })

    expect(writeText).toHaveBeenCalledWith('書き直した合成の文章です。')
    expect(screen.getByText(/コピーできませんでした/)).toBeInTheDocument()
    expect(screen.queryByTestId('CheckIcon')).not.toBeInTheDocument()
  })

  it('コピー表示を戻すタイマーはアンマウント時に解除される', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REWRITE_RESULT })
    const { unmount } = render(<ESRewritePage />)

    submitRewrite('元の合成文章です。')
    await screen.findByText('リライト後')

    // MUI 側のタイマーも動くため、2秒の戻しタイマーだけを追う
    const setTimeoutSpy = jest.spyOn(window, 'setTimeout')
    const clearTimeoutSpy = jest.spyOn(window, 'clearTimeout')
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'リライト結果をコピー' }))
    })
    expect(screen.getByTestId('CheckIcon')).toBeInTheDocument()
    const index = setTimeoutSpy.mock.calls.findIndex(([, delay]) => delay === 2000)
    expect(index).toBeGreaterThanOrEqual(0)
    const timerId = setTimeoutSpy.mock.results[index].value

    unmount()
    expect(clearTimeoutSpy).toHaveBeenCalledWith(timerId)
  })
})
