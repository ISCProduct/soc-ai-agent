/** @jest-environment jsdom */

import { createRef, type ComponentProps } from 'react'
import { render, screen } from '@testing-library/react'
import { ChatInputBar } from '@/components/mui-chat/components/ChatInputBar'
import type { ChoiceOption } from '@/components/mui-chat/types'

/**
 * 選択肢が横並びの大きなタイルになり、入力欄と合わせて画面を埋める問題（#1763）。
 * 1列のリストと、選択肢表示中は入力欄を1行にすることを固定する。
 */

const choices: ChoiceOption[] = [
  { value: 'A', label: 'A', text: '自分から新しいことを学び続けたい' },
  { value: 'B', label: 'B', text: '必要になったときに学べばいい' },
  { value: 'C', label: 'C', text: 'その他（自由記述）' },
]

function renderBar(overrides: Partial<ComponentProps<typeof ChatInputBar>> = {}) {
  return render(
    <ChatInputBar
      analysisComplete={false}
      showChoiceButtons
      choiceOptions={choices}
      selectedChoiceValue={null}
      input=""
      inputPlaceholder="選択肢を選ぶか、同じ内容を入力してください"
      isLoading={false}
      historyLoadError={null}
      canSend={false}
      inputRef={createRef()}
      onInputChange={() => {}}
      onSelectChoice={() => {}}
      onSend={() => {}}
      onOtherChoice={() => {}}
      onShowCompletionModal={() => {}}
      {...overrides}
    />,
  )
}

describe('チャット選択肢の並び', () => {
  it('選択肢は横並びではなく1列のグループになる', () => {
    renderBar()
    const group = screen.getByRole('group', { name: '回答の選択肢' })
    expect(group).toHaveStyle({ flexDirection: 'column' })
    expect(screen.getAllByRole('button', { pressed: false })).toHaveLength(3)
  })

  it('選んだ選択肢は押された状態として読める', () => {
    renderBar({ selectedChoiceValue: 'A' })
    expect(
      screen.getByRole('button', { name: /自分から新しいことを学び続けたい/ }),
    ).toHaveAttribute('aria-pressed', 'true')
    expect(
      screen.getByRole('button', { name: /必要になったときに学べばいい/ }),
    ).toHaveAttribute('aria-pressed', 'false')
  })

  it('選択肢表示中は送信の補足を短くする', () => {
    renderBar()
    expect(screen.getByText('Ctrl+Enterで送信')).toBeTruthy()
  })

  it('自由記述のときは改行の補足を出し、選択肢は出さない', () => {
    renderBar({
      showChoiceButtons: false,
      choiceOptions: [],
      inputPlaceholder: 'メッセージを入力...',
    })
    expect(screen.queryByRole('group', { name: '回答の選択肢' })).toBeNull()
    expect(screen.getByText(/改行: Enter/)).toBeTruthy()
  })
})
