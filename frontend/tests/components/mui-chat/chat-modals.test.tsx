/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { ChatModals } from '@/components/mui-chat/components/ChatModals'

// #2212: 強制終了モーダルで、キーボードだけで使う人が行き詰まらないこと。
// 意図して閉じない作りは残し、閉じられない理由を文で示し、押せる操作を必ず1つ残す。
const baseProps = {
  showCompletionModal: false,
  showEndChatModal: false,
  showTerminationModal: false,
  allPhasesCompleted: false,
  onContinueChat: jest.fn(),
  onViewResults: jest.fn(),
  onCancelEndChat: jest.fn(),
  onConfirmEndChat: jest.fn(),
}

describe('ChatModals 強制終了モーダル', () => {
  const setup = () => {
    const onStartNewSession = jest.fn()
    render(<ChatModals {...baseProps} showTerminationModal onStartNewSession={onStartNewSession} />)
    return { onStartNewSession, button: screen.getByRole('button', { name: '新しいセッションを開始' }) }
  }

  it('「新しいセッションを開始」が Tab で届く（tabIndex で外していない）', () => {
    const { button } = setup()
    expect(button).not.toHaveAttribute('tabindex', '-1')
  })

  it('開いたときに「新しいセッションを開始」にフォーカスが移る', () => {
    const { button } = setup()
    expect(button).toHaveFocus()
  })

  it('押すと新しいセッションを始める', () => {
    const { button, onStartNewSession } = setup()
    fireEvent.click(button)
    expect(onStartNewSession).toHaveBeenCalledTimes(1)
  })

  it('Esc では閉じないので、閉じられない理由を画面の文で示す', () => {
    setup()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText(/この画面は閉じられません/)).toBeInTheDocument()
  })

  it('既定の「新しいセッションを開始」は、いまのセッションの記録を消す', () => {
    sessionStorage.setItem('chatSessionId', '12')
    localStorage.setItem('currentSessionId', '12')
    render(<ChatModals {...baseProps} showTerminationModal />)
    // jsdom は読み込み直し（location.reload）を実装していないので、エラーの出力だけ抑える
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    fireEvent.click(screen.getByRole('button', { name: '新しいセッションを開始' }))
    spy.mockRestore()
    expect(sessionStorage.getItem('chatSessionId')).toBeNull()
    expect(localStorage.getItem('currentSessionId')).toBeNull()
  })
})

describe('ChatModals 分析完了モーダル（全フェーズ完了）', () => {
  it('「結果を見る」がキーボードで届き、押すと結果へ進む', () => {
    const onViewResults = jest.fn()
    render(<ChatModals {...baseProps} showCompletionModal allPhasesCompleted onViewResults={onViewResults} />)
    const button = screen.getByRole('button', { name: '結果を見る' })
    expect(button).not.toHaveAttribute('tabindex', '-1')
    expect(screen.queryByRole('button', { name: 'チャットを続ける' })).not.toBeInTheDocument()
    fireEvent.click(button)
    expect(onViewResults).toHaveBeenCalledTimes(1)
  })
})
