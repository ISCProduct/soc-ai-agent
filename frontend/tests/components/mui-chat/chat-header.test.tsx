/** @jest-environment jsdom */
import { fireEvent, render, screen } from '@testing-library/react'
import { ChatHeader } from '@/components/mui-chat/components/ChatHeader'

const totals = { valid: 12, required: 15, percent: 64 }

describe('ChatHeader（判定の進み具合）', () => {
  it('回答数ではなく、判定の進み具合を出す', () => {
    render(<ChatHeader progressTotals={totals} questionCount={12} totalQuestions={15} onEndChat={jest.fn()} />)
    expect(screen.getByText('判定の進み具合 64%')).toBeInTheDocument()
    expect(screen.queryByText(/問に回答/)).toBeNull()
  })

  it('目安の回答数を超えても判定できないときだけ、暫定の結果を見る導線を出す', () => {
    const onViewProvisional = jest.fn()
    const { rerender } = render(
      <ChatHeader progressTotals={totals} questionCount={30} totalQuestions={15} onEndChat={jest.fn()} onViewProvisional={onViewProvisional} />,
    )
    expect(screen.queryByRole('button', { name: 'いまの暫定の結果を見る' })).toBeNull()

    rerender(
      <ChatHeader
        progressTotals={totals}
        questionCount={30}
        totalQuestions={15}
        suggestProvisional
        onEndChat={jest.fn()}
        onViewProvisional={onViewProvisional}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'いまの暫定の結果を見る' }))
    expect(onViewProvisional).toHaveBeenCalledTimes(1)
  })
})
