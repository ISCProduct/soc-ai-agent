/**
 * @jest-environment jsdom
 */
import { act, render, screen } from '@testing-library/react'
import { ResumeRunningCard, formatElapsed } from '@/app/resume/components/ResumeRunningCard'

function renderCard(progress: { stage: number; percent: number } | null) {
  return render(
    <ResumeRunningCard progress={progress} fileName="履歴書.pdf" targetLabel="サンプル株式会社" candidateLabel="新卒" onCancel={() => {}} />,
  )
}

describe('ResumeRunningCard', () => {
  afterEach(() => jest.useRealTimers())

  it('進み具合の％・段階・状態の文言を出す', () => {
    renderCard({ stage: 2, percent: 70 })
    const bar = screen.getByRole('progressbar', { name: 'レビューの進み具合' })
    expect(bar).toHaveAttribute('aria-valuenow', '70')
    expect(screen.getByRole('status')).toHaveTextContent('レポートを書いています')
    expect(screen.getByText('全体の進み具合（3 / 4 段階目）')).toBeInTheDocument()
    // 色だけで示さず、文言で状態を伝える
    expect(screen.getAllByText('完了')).toHaveLength(2)
    expect(screen.getAllByText('進行中')).toHaveLength(1)
    expect(screen.getAllByText('待機中')).toHaveLength(1)
  })

  it('進捗を受け取る前は 0% から始める', () => {
    renderCard(null)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getByRole('status')).toHaveTextContent('書類を読み取っています')
  })

  it('目安の60秒を超えたら「お知らせ」を出す', () => {
    jest.useFakeTimers()
    renderCard({ stage: 3, percent: 85 })
    expect(screen.queryByText('お知らせ')).not.toBeInTheDocument()
    act(() => {
      jest.advanceTimersByTime(61_000)
    })
    expect(screen.getByText('お知らせ')).toBeInTheDocument()
    expect(screen.getByText('目安 30〜60秒を超えています')).toBeInTheDocument()
  })
})

describe('formatElapsed', () => {
  it.each([
    [0, '0秒'],
    [41, '41秒'],
    [78, '1分18秒'],
  ])('%i 秒 → %s', (sec, want) => {
    expect(formatElapsed(sec)).toBe(want)
  })
})
