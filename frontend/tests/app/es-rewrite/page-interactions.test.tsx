/**
 * @jest-environment jsdom
 */
import { act, render, screen, fireEvent } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page-content'

// 部品分割（#2569）の前に、既存テストが触れていない操作を固定する。
const mockPush = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
  useSearchParams: () => new URLSearchParams(),
}))

const ES_PLACEHOLDER = /チームで開発した経験があります/

const REWRITE_RESULT = {
  rewritten_text: '書き直した合成の文章です。',
  star: { situation: '状況の例', task: '課題の例', action: '', result: '成果の例' },
}

const REVIEW_RESULT = {
  specificity_score: 7,
  star_score: 6,
  company_fit_score: null,
  length_balance_score: 5,
  feedback: '合成のフィードバック',
  improved_text: '改善した合成の文章です。',
}

describe('ESRewritePage 操作の固定 (#2569)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
    mockPush.mockClear()
  })

  afterEach(() => {
    jest.restoreAllMocks()
    jest.useRealTimers()
  })

  it('初期表示はリライトモードで、ヘッダーの戻るボタンでホームへ遷移する', () => {
    render(<ESRewritePage />)

    expect(screen.getByText('ESリライト — 元の文章を入力')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '書き直す' })).toBeDisabled()
    expect(screen.getByLabelText(/使用技術スタック/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'ホームへ戻る' }))
    expect(mockPush).toHaveBeenCalledWith('/')
  })

  it('リライト結果を表示し、STARの空欄は—、元の文章も出す。添削へ切り替えると結果を消す', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REWRITE_RESULT })
    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '元の合成文章です。' } })
    fireEvent.change(screen.getByLabelText(/使用技術スタック/), { target: { value: 'React' } })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    await screen.findByText('リライト後')
    expect((global.fetch as jest.Mock).mock.calls).toHaveLength(1)
    const [url, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, { body: string }]
    expect(url).toBe('/api/es/rewrite')
    expect(JSON.parse(init.body)).toEqual({
      original_text: '元の合成文章です。',
      question_type: '学チカ',
      tech_stack: 'React',
      company_name: '',
    })
    expect(screen.getByText('書き直した合成の文章です。')).toBeInTheDocument()
    expect(screen.getByText('状況の例')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.getByText('元の文章')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    expect(screen.queryByText('リライト後')).not.toBeInTheDocument()
    expect(screen.getByText('ES添削 — 元の文章を入力')).toBeInTheDocument()
    expect(screen.queryByLabelText(/使用技術スタック/)).not.toBeInTheDocument()
  })

  it('質問種別のチップを選ぶと送信内容に反映される', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REVIEW_RESULT })
    render(<ESRewritePage />)

    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.click(screen.getByText('自己PR'))
    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '元の合成文章です。' } })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    await screen.findByText('添削スコア')
    const [url, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, { body: string }]
    expect(url).toBe('/api/es/review')
    expect(JSON.parse(init.body)).toEqual({
      es_text: '元の合成文章です。',
      question: '',
      question_type: '自己PR',
      company_name: '',
    })
  })

  it('エラーはモード切替で消える', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: false, json: async () => ({ error: '合成のエラー文' }) })
    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '元の合成文章です。' } })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))
    expect(await screen.findByText('合成のエラー文')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    expect(screen.queryByText('合成のエラー文')).not.toBeInTheDocument()
  })

  it('【 】が残っていると警告を出す', () => {
    render(<ESRewritePage />)
    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '私は【経験】をしました。' } })
    expect(screen.getByText(/【 】の部分が残っています/)).toBeInTheDocument()
  })

  it('改善文をコピーすると2秒だけ「コピーしました」に変わる', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => REVIEW_RESULT })
    render(<ESRewritePage />)

    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(ES_PLACEHOLDER), { target: { value: '元の合成文章です。' } })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))
    await screen.findByText('添削スコア')

    jest.useFakeTimers()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '改善文をコピー' }))
    })
    expect(writeText).toHaveBeenCalledWith('改善した合成の文章です。')
    expect(screen.getByTestId('CheckIcon')).toBeInTheDocument()

    act(() => {
      jest.advanceTimersByTime(2000)
    })
    expect(screen.queryByTestId('CheckIcon')).not.toBeInTheDocument()
    expect(screen.getByTestId('ContentCopyIcon')).toBeInTheDocument()
  })
})
