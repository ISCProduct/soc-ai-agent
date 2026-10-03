/** @jest-environment jsdom */

import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import { ChatMessageList } from '@/components/mui-chat/components/ChatMessageList'
import type { Message } from '@/components/mui-chat/types'

/**
 * 履歴の読み込み中に画面が空白になる問題（§23）の回帰テスト。
 *
 * 履歴が0件でも挨拶メッセージが入るので、messages が空なのは初回取得中だけ。
 * そのあいだ何も描かないと、通信が遅い利用者には壊れているようにしか見えない。
 */

function baseProps() {
  return {
    messages: [] as Message[],
    isLoading: false,
    historyLoadError: null,
    historyRetrying: false,
    messagesEndRef: createRef<HTMLDivElement>(),
    messagesAreaRef: createRef<HTMLDivElement>(),
    onRetryHistoryLoad: jest.fn(),
    onQuickSelect: jest.fn(),
  }
}

const greeting: Message = {
  id: '0',
  role: 'assistant',
  content: 'こんにちは',
  timestamp: new Date('2026-10-01T00:00:00Z'),
}

/** Skeleton は MUI が .MuiSkeleton-root を付けるので、それを数える。 */
function skeletonCount(container: HTMLElement): number {
  return container.querySelectorAll('.MuiSkeleton-root').length
}

describe('ChatMessageList の読み込み表示', () => {
  it('履歴の読み込み中は吹き出しの骨格を出す', () => {
    const { container } = render(<ChatMessageList {...baseProps()} historyLoading />)
    expect(skeletonCount(container)).toBeGreaterThan(0)
  })

  it('読み込みが終わったら骨格を消す', () => {
    const { container } = render(
      <ChatMessageList {...baseProps()} historyLoading={false} messages={[greeting]} />,
    )
    expect(skeletonCount(container)).toBe(0)
    expect(screen.getByText('こんにちは')).toBeInTheDocument()
  })

  it('メッセージが届いていれば読み込み中でも骨格は出さない', () => {
    // 再取得などで両方 true になり得る。骨格と本文が並ぶと二重に見える。
    const { container } = render(
      <ChatMessageList {...baseProps()} historyLoading messages={[greeting]} />,
    )
    expect(skeletonCount(container)).toBe(0)
  })

  it('履歴エラーのときは骨格ではなくエラーを出す', () => {
    // 骨格を出し続けると、失敗したのに読み込み中に見える。
    const { container } = render(
      <ChatMessageList
        {...baseProps()}
        historyLoading
        historyLoadError="履歴の読み込みに失敗しました。"
      />,
    )
    expect(skeletonCount(container)).toBe(0)
    expect(screen.getByText('履歴の読み込みに失敗しました。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再試行' })).toBeInTheDocument()
  })

  it('一覧は読み上げへ追加を通知する', () => {
    // AI応答が届いたことを読み上げ環境へ伝える（既存の挙動を固定する）。
    const { container } = render(<ChatMessageList {...baseProps()} messages={[greeting]} />)
    const log = container.querySelector('[role="log"]')
    expect(log).not.toBeNull()
    expect(log?.getAttribute('aria-live')).toBe('polite')
  })
})

describe('読み上げでの区別（WCAG）', () => {
  const at = new Date('2026-10-01T09:05:00')

  it('一覧に名前が付いている', () => {
    // 名前が無いと読み上げでは「ログ」としか案内されない。
    const { container } = render(<ChatMessageList {...baseProps()} messages={[greeting]} />)
    expect(container.querySelector('[role="log"]')?.getAttribute('aria-label')).toBe(
      '自己分析チャットのやり取り',
    )
  })

  it('各メッセージが発言者と時刻を持つ区切りになっている', () => {
    // 位置と色でしか区別していなかったため、支援技術では誰の発言か分からなかった。
    render(
      <ChatMessageList
        {...baseProps()}
        messages={[
          { id: 'a', role: 'assistant', content: '質問です', timestamp: at },
          { id: 'b', role: 'user', content: '回答です', timestamp: at },
        ]}
      />,
    )
    expect(screen.getByRole('article', { name: 'エージェント、9時05分' })).toBeInTheDocument()
    expect(screen.getByRole('article', { name: 'あなた、9時05分' })).toBeInTheDocument()
  })

  it('時刻が壊れていても発言者は伝える', () => {
    render(
      <ChatMessageList
        {...baseProps()}
        messages={[{ id: 'a', role: 'user', content: 'x', timestamp: new Date('invalid') }]}
      />,
    )
    expect(screen.getByRole('article', { name: 'あなた' })).toBeInTheDocument()
  })
})
