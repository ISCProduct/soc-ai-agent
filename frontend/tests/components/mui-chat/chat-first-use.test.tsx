/** @jest-environment jsdom */

import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import { ChatMessageList } from '@/components/mui-chat/components/ChatMessageList'
import { INITIAL_GREETING } from '@/components/mui-chat/utils'
import type { Message } from '@/components/mui-chat/types'

/**
 * 初回利用の空状態（§24）。
 *
 * まだ1問も答えていない状態では、候補のチップだけが並んでいた。
 * チップは押せば送られることが見た目で分かる一方、
 *
 * - 今どういう状態なのか（まだ回答が無い）
 * - 選ぶと何が起きるのか
 * - 当てはまるものが無いときはどうするのか
 *
 * がどこにも書かれておらず、初めて開いた学生は入力欄とチップのどちらを
 * 使えばいいのか判断できない。§24 の「理由 + 次の操作」を満たす。
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
  content: 'まず、どんな仕事に興味がありますか？',
  timestamp: new Date('2026-10-01T09:05:00'),
}

const answered: Message = {
  id: '1',
  role: 'user',
  content: '開発系エンジニア',
  timestamp: new Date('2026-10-01T09:06:00'),
}

describe('初回利用の空状態', () => {
  it('まだ回答が無いときは状態と次の操作を示す', () => {
    render(<ChatMessageList {...baseProps()} messages={[greeting]} />)

    expect(screen.getByText('まだ回答がありません')).toBeInTheDocument()
    // 「データがありません」で終わらせない。次に何をすればよいかを書く（§24）。
    expect(
      screen.getByText(/当てはまるものが無ければ、下の入力欄に書いて送ってください/),
    ).toBeInTheDocument()
  })

  it('候補のチップは名前付きのまとまりとして読み上げへ渡す', () => {
    // 見出しは置かない（機能紹介に見えるため以前外した経緯がある）。
    // 代わりに ChatInputBar の選択肢と同じく role="group" + 名前で伝える。
    render(<ChatMessageList {...baseProps()} messages={[greeting]} />)

    const group = screen.getByRole('group', { name: '興味のある仕事の候補' })
    expect(group).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '開発系エンジニア' })).toBeInTheDocument()
  })

  it('1問でも答えていれば空状態は出さない', () => {
    render(<ChatMessageList {...baseProps()} messages={[greeting, answered]} />)

    expect(screen.queryByText('まだ回答がありません')).toBeNull()
    expect(screen.queryByRole('group', { name: '興味のある仕事の候補' })).toBeNull()
  })

  it('履歴エラーのときは空状態ではなくエラーを出す', () => {
    render(
      <ChatMessageList
        {...baseProps()}
        messages={[greeting]}
        historyLoadError="履歴の読み込みに失敗しました。"
      />,
    )

    expect(screen.queryByText('まだ回答がありません')).toBeNull()
    expect(screen.getByText('履歴の読み込みに失敗しました。')).toBeInTheDocument()
  })
})

describe('初回利用の空状態を出さない場面', () => {
  it('その質問に自前の選択肢があるときは出さない', () => {
    // 選択肢付きの質問は ChatInputBar 側に A)〜D) が並ぶ。
    // そこへ「近いものを選ぶと…」と職種の候補を重ねると、
    // 質問と関係のない候補を選ばせることになる。
    const withChoices: Message = {
      id: '0',
      role: 'assistant',
      content: [
        '新しいことに挑戦しましたか？',
        '',
        'A) 自分から新しいことを学び続けたい',
        'B) 必要になったときに学べばいい',
      ].join('\n'),
      timestamp: new Date('2026-10-01T09:05:00'),
    }

    render(<ChatMessageList {...baseProps()} messages={[withChoices]} />)

    expect(screen.queryByText('まだ回答がありません')).toBeNull()
    expect(screen.queryByRole('group', { name: '興味のある仕事の候補' })).toBeNull()
  })

  it('実際の挨拶文では空状態を出す', () => {
    // 挨拶に番号付きの行が混ざると選択肢として拾われ、空状態が消える。
    const actual: Message = {
      id: '0',
      role: 'assistant',
      content: INITIAL_GREETING,
      timestamp: new Date('2026-10-01T09:05:00'),
    }

    render(<ChatMessageList {...baseProps()} messages={[actual]} />)

    expect(screen.getByText('まだ回答がありません')).toBeInTheDocument()
  })
})

describe('読み込み中のライブリージョン', () => {
  it('履歴を読み込んでいるあいだは aria-busy を立てる', () => {
    // 骨格の追加を「新しい発言」として読み上げさせない。
    // 共通の ListSkeleton と同じ扱いに揃える。
    const { container } = render(<ChatMessageList {...baseProps()} historyLoading />)
    expect(container.querySelector('[role="log"]')?.getAttribute('aria-busy')).toBe('true')
  })

  it('読み込みが終われば aria-busy を下ろす', () => {
    const { container } = render(
      <ChatMessageList {...baseProps()} historyLoading={false} messages={[greeting]} />,
    )
    expect(container.querySelector('[role="log"]')?.getAttribute('aria-busy')).toBe('false')
  })
})
