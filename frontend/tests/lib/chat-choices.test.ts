import { resolveChatOutgoingMessage, type ChatChoiceOption } from '@/lib/chat-choices'

const choices: ChatChoiceOption[] = [
  { value: '1', label: '1', text: '新しい技術やツールに触れる' },
  { value: '2', label: '2', text: '仕組みを考えたり設計する' },
  { value: '3', label: '3', text: '人と関わりながら進める' },
  { value: '5', label: '5', text: 'その他（自由記述）' },
]

describe('resolveChatOutgoingMessage', () => {
  it('選択肢記号はそのまま返す', () => {
    expect(resolveChatOutgoingMessage('1', choices, false)).toBe('1')
    expect(resolveChatOutgoingMessage('a', [{ value: 'A', label: 'A', text: '主導' }], false)).toBe('A')
  })

  it('選択肢ラベルの自由入力を記号へ正規化する', () => {
    expect(resolveChatOutgoingMessage('新しい技術やツールに触れる', choices, false)).toBe('1')
    expect(resolveChatOutgoingMessage('仕組みを考えたり設計する', choices, false)).toBe('2')
  })

  it('その他活性時や不一致の自由記述は原文のまま', () => {
    expect(resolveChatOutgoingMessage('自分のペースで学びたい', choices, true)).toBe(
      '自分のペースで学びたい',
    )
    expect(resolveChatOutgoingMessage('自分のペースで学びたい', choices, false)).toBe(
      '自分のペースで学びたい',
    )
  })

  it('選択肢が無いときは原文のまま', () => {
    expect(resolveChatOutgoingMessage('こんにちは', [], false)).toBe('こんにちは')
  })
})
