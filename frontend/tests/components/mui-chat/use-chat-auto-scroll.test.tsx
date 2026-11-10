/** @jest-environment jsdom */

import { render } from '@testing-library/react'
import { useChatAutoScroll } from '@/components/mui-chat/hooks/useChatAutoScroll'
import type { Message } from '@/components/mui-chat/types'

/**
 * 自動スクロールの prefers-reduced-motion 対応（§22）。
 *
 * 返答が届くたびに smooth で流れると、前庭障害や光過敏のある利用者には
 * 繰り返し動く要素と同じ負荷になる。動きを減らす設定のときは一瞬で移動させる。
 * 移動そのものは止めない。止めると最新の発言が画面外に残り、情報を失う。
 */

function setMatchMedia(reduce: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: reduce && query.includes('prefers-reduced-motion'),
      media: query,
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    }),
  })
}

function Harness({ messages, isLoading }: { messages: Message[]; isLoading: boolean }) {
  const { messagesEndRef, messagesAreaRef } = useChatAutoScroll(messages, isLoading)
  return (
    <div ref={messagesAreaRef}>
      <div ref={messagesEndRef} />
    </div>
  )
}

const messages: Message[] = [
  { id: 'a', role: 'assistant', content: '質問です', timestamp: new Date('2026-10-01T09:05:00') },
]

describe('useChatAutoScroll', () => {
  let scrollIntoView: jest.Mock

  beforeEach(() => {
    scrollIntoView = jest.fn()
    Element.prototype.scrollIntoView = scrollIntoView
  })

  it('通常は smooth で最新の発言まで送る', () => {
    setMatchMedia(false)
    render(<Harness messages={messages} isLoading={false} />)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth' })
  })

  it('動きを減らす設定では滑らせずに移動する', () => {
    setMatchMedia(true)
    render(<Harness messages={messages} isLoading={false} />)
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'auto' })
  })

  it('動きを減らす設定でも移動自体は止めない', () => {
    // アニメーションを消すだけにする。移動まで止めると最新の発言が見えない。
    setMatchMedia(true)
    render(<Harness messages={messages} isLoading={false} />)
    expect(scrollIntoView).toHaveBeenCalled()
  })

  it('利用者が上へ遡っているあいだは追わない', () => {
    setMatchMedia(false)
    const { container, rerender } = render(<Harness messages={messages} isLoading={false} />)
    scrollIntoView.mockClear()

    const area = container.firstElementChild as HTMLElement
    Object.defineProperty(area, 'scrollHeight', { value: 2000, configurable: true })
    Object.defineProperty(area, 'clientHeight', { value: 500, configurable: true })
    area.scrollTop = 0

    rerender(<Harness messages={[...messages, messages[0]]} isLoading />)
    expect(scrollIntoView).not.toHaveBeenCalled()
  })
})
