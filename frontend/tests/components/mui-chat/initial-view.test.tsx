/** @jest-environment jsdom */
import { render, screen } from '@testing-library/react'
import { MuiChat } from '@/components/mui-chat'

const mockChat: Record<string, unknown> = {}
jest.mock('@/components/mui-chat/hooks/useMuiChat', () => ({ useMuiChat: () => mockChat }))
jest.mock('@/components/mui-chat/components/ChatHeader', () => ({ ChatHeader: () => <div data-testid="chat-header" /> }))
jest.mock('@/components/mui-chat/components/ChatMessageList', () => ({ ChatMessageList: () => <div data-testid="chat-list" /> }))
jest.mock('@/components/mui-chat/components/ChatInputBar', () => ({ ChatInputBar: () => <div data-testid="chat-input" /> }))
jest.mock('@/components/mui-chat/components/ChatModals', () => ({ ChatModals: () => null }))

const greeting = { id: '0', role: 'assistant', content: 'g', timestamp: new Date() }
const reply = { id: '1', role: 'user', content: '開発', timestamp: new Date() }

function setChat(overrides: Record<string, unknown>) {
  for (const k of Object.keys(mockChat)) delete mockChat[k]
  Object.assign(mockChat, {
    mounted: true,
    messages: [greeting],
    isLoading: false,
    historyLoading: false,
    historyLoadError: null,
    analysisComplete: false,
    progressTotals: { valid: 2, required: 15, percent: 13 },
    handleSend: jest.fn(),
    ...overrides,
  })
}

describe('MuiChat の initialView（診断を始める前の表示の差し替え）', () => {
  const initialView = jest.fn(() => <div data-testid="initial-view" />)
  beforeEach(() => initialView.mockClear())

  it('履歴が0件（案内のメッセージだけ）なら、チャットの代わりに initialView を出す', () => {
    setChat({})
    render(<MuiChat initialView={initialView} />)
    expect(screen.getByTestId('initial-view')).toBeInTheDocument()
    expect(screen.queryByTestId('chat-header')).toBeNull()
  })

  it('initialView には、送信・送信中・進捗を渡す。onSend は handleSend へ回答を渡す', () => {
    const handleSend = jest.fn()
    setChat({ handleSend, isLoading: true })
    render(<MuiChat initialView={initialView} />)
    const ctx = (initialView.mock.calls[0] as unknown[])[0] as { onSend: (t: string, intent?: string) => void; isLoading: boolean; progress: unknown }
    expect(ctx.isLoading).toBe(true)
    expect(ctx.progress).toEqual({ valid: 2, required: 15, percent: 13 })
    ctx.onSend('自分の強みを整理したい', 'organize_strengths')
    expect(handleSend).toHaveBeenCalledWith('自分の強みを整理したい', { consultIntent: 'organize_strengths' })
  })

  it('1通でも送ったあとは、従来のチャット表示に切り替わる', () => {
    setChat({ messages: [greeting, reply] })
    render(<MuiChat initialView={initialView} />)
    expect(screen.queryByTestId('initial-view')).toBeNull()
    expect(screen.getByTestId('chat-header')).toBeInTheDocument()
    expect(screen.getByTestId('chat-input')).toBeInTheDocument()
  })

  it.each([
    ['履歴を読み込み中', { historyLoading: true }],
    ['履歴の読み込みに失敗', { historyLoadError: 'error' }],
    ['診断が完了済み', { analysisComplete: true }],
  ])('%sのときは、従来のチャット表示のまま（エラーや進捗を隠さない）', (_name, overrides) => {
    setChat(overrides)
    render(<MuiChat initialView={initialView} />)
    expect(screen.queryByTestId('initial-view')).toBeNull()
    expect(screen.getByTestId('chat-list')).toBeInTheDocument()
  })

  it('initialView を渡さなければ、これまでどおりのチャット表示', () => {
    setChat({})
    render(<MuiChat />)
    expect(screen.getByTestId('chat-header')).toBeInTheDocument()
  })
})
