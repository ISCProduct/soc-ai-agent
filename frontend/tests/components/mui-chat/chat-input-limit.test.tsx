/** @jest-environment jsdom */

import { createRef, type ComponentProps } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { ChatInputBar } from '@/components/mui-chat/components/ChatInputBar'
import { CHAT_MESSAGE_MAX_CHARS } from '@/components/mui-chat/utils'

/**
 * チャット本文の文字数上限（SOCAIAGENT-702）。
 * 入力欄に「n / 上限」を出し、超えたら送信を止めて入力欄の下にエラーを出す。
 */

function renderBar(overrides: Partial<ComponentProps<typeof ChatInputBar>> = {}) {
  const onSend = jest.fn()
  render(
    <ChatInputBar
      analysisComplete={false}
      showChoiceButtons={false}
      choiceOptions={[]}
      selectedChoiceValue={null}
      input=""
      inputPlaceholder="メッセージを入力..."
      isLoading={false}
      historyLoadError={null}
      canSend
      inputRef={createRef()}
      onInputChange={() => {}}
      onSelectChoice={() => {}}
      onSend={onSend}
      onOtherChoice={() => {}}
      onShowCompletionModal={() => {}}
      {...overrides}
    />,
  )
  return { onSend }
}

describe('チャット本文の文字数上限', () => {
  it('上限は 2000 字', () => {
    expect(CHAT_MESSAGE_MAX_CHARS).toBe(2000)
  })

  it('入力中の文字数と上限を出す（絵文字・日本語は1字として数える）', () => {
    renderBar({ input: 'あ😀' })
    expect(screen.getByText(`2 / ${CHAT_MESSAGE_MAX_CHARS}`)).toBeInTheDocument()
  })

  it('上限ちょうどは送信できる', () => {
    const { onSend } = renderBar({ input: 'あ'.repeat(CHAT_MESSAGE_MAX_CHARS) })
    expect(screen.queryByText(/文字数が上限を超えています/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'メッセージを送信' }))
    expect(onSend).toHaveBeenCalledTimes(1)
  })

  it('上限を超えたら送信ボタンもキー操作も止め、入力欄の下にエラーを出す', () => {
    const { onSend } = renderBar({ input: 'あ'.repeat(CHAT_MESSAGE_MAX_CHARS + 1) })
    expect(screen.getByText(/文字数が上限を超えています/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'メッセージを送信' })).toBeDisabled()
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter', ctrlKey: true })
    expect(onSend).not.toHaveBeenCalled()
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })
})
