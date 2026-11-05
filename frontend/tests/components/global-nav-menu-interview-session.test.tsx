/** @jest-environment jsdom */

import { act, render } from '@testing-library/react'
import { GlobalNavMenu } from '@/components/GlobalNavMenu'
import { setInterviewSessionActive } from '@/app/interview/sessionChrome'

/**
 * AI面接のセッション画面（SessionScreen）表示中は、画面右下に固定表示される
 * メニューFabを隠すこと（#1569）。StudentBottomNavと同じ理由で、Fabが
 * 「話す」「完了する」ボタンに重なって操作できなくなるのを防ぐ。
 */

jest.mock('next/navigation', () => ({
  usePathname: () => '/interview',
}))

describe('GlobalNavMenu とAI面接セッションの連動 (#1569)', () => {
  beforeEach(() => {
    setInterviewSessionActive(false)
  })

  afterEach(() => {
    setInterviewSessionActive(false)
  })

  it('セッション画面表示中(setInterviewSessionActive(true))はFabを描画しない', () => {
    const { queryByLabelText } = render(<GlobalNavMenu />)
    expect(queryByLabelText('メニューを開く')).toBeInTheDocument()

    act(() => {
      setInterviewSessionActive(true)
    })

    expect(queryByLabelText('メニューを開く')).not.toBeInTheDocument()

    act(() => {
      setInterviewSessionActive(false)
    })

    expect(queryByLabelText('メニューを開く')).toBeInTheDocument()
  })
})
