/** @jest-environment jsdom */

import { act, renderHook } from '@testing-library/react'
import { useSpeechInput } from '@/components/mui-chat/hooks/useSpeechInput'

/**
 * 音声入力。ブラウザ内蔵の音声認識を使うので、認識器を差し替えて挙動を固定する。
 *
 * 認識結果は自動送信しない。音声認識は誤りが珍しくないため、
 * 呼び出し側が入力欄へ入れて直せる状態にする前提になっている。
 */

type Handlers = {
  onresult: ((e: unknown) => void) | null
  onerror: ((e: { error: string }) => void) | null
  onend: (() => void) | null
}

/** 直近に生成された認識器を掴むための入れ物。 */
let latest: FakeRecognition | null = null

class FakeRecognition {
  lang = ''
  continuous = false
  interimResults = false
  onresult: Handlers['onresult'] = null
  onerror: Handlers['onerror'] = null
  onend: Handlers['onend'] = null
  started = 0
  stopped = 0
  aborted = 0

  constructor() {
    latest = this
  }
  start() {
    this.started++
  }
  stop() {
    this.stopped++
  }
  abort() {
    this.aborted++
  }
}

/** 確定結果のイベントを組み立てる。 */
function finalResult(text: string) {
  return {
    resultIndex: 0,
    results: Object.assign([Object.assign([{ transcript: text }], { isFinal: true })], {
      length: 1,
    }),
  }
}

function installRecognition() {
  ;(window as unknown as Record<string, unknown>).SpeechRecognition = FakeRecognition
}

function removeRecognition() {
  delete (window as unknown as Record<string, unknown>).SpeechRecognition
  delete (window as unknown as Record<string, unknown>).webkitSpeechRecognition
}

describe('useSpeechInput', () => {
  beforeEach(() => {
    latest = null
    removeRecognition()
  })
  afterEach(removeRecognition)

  it('使えない環境では supported が false', () => {
    // ボタン自体を出さないための判定。押せるのに動かない状態を避ける。
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    expect(result.current.supported).toBe(false)
  })

  it('webkit 接頭辞だけでも使えると判定する（Safari）', () => {
    ;(window as unknown as Record<string, unknown>).webkitSpeechRecognition = FakeRecognition
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    expect(result.current.supported).toBe(true)
  })

  it('日本語で、確定結果だけを返す設定にする', () => {
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    expect(latest?.lang).toBe('ja-JP')
    // 確定前の文字を入力欄へ入れると、書き換わって直しにくい。
    expect(latest?.interimResults).toBe(false)
  })

  it('確定した文字列をコールバックへ渡す', () => {
    installRecognition()
    const onText = jest.fn()
    const { result } = renderHook(() => useSpeechInput(onText))
    act(() => result.current.start())
    act(() => latest?.onresult?.(finalResult('エンジニアに興味があります')))
    expect(onText).toHaveBeenCalledWith('エンジニアに興味があります')
  })

  it('空の結果では呼ばない', () => {
    installRecognition()
    const onText = jest.fn()
    const { result } = renderHook(() => useSpeechInput(onText))
    act(() => result.current.start())
    act(() => latest?.onresult?.(finalResult('   ')))
    expect(onText).not.toHaveBeenCalled()
  })

  it('連打しても認識器を重ねない', () => {
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    const first = latest
    act(() => result.current.start())
    // 1回目は畳まれている
    expect(first?.aborted).toBe(1)
  })

  it('マイク拒否は、次に何をすればよいか分かる文言にする', () => {
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    act(() => latest?.onerror?.({ error: 'not-allowed' }))
    expect(result.current.error).toContain('マイクの使用が許可されていません')
    expect(result.current.listening).toBe(false)
  })

  it('未知のエラーでもキーボード入力できることを伝える', () => {
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    act(() => latest?.onerror?.({ error: 'something-new' }))
    expect(result.current.error).toContain('キーボード')
  })

  it('終了したら listening を下ろす', () => {
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    expect(result.current.listening).toBe(true)
    act(() => latest?.onend?.())
    expect(result.current.listening).toBe(false)
  })

  it('画面を離れるときにマイクを握ったままにしない', () => {
    installRecognition()
    const { result, unmount } = renderHook(() => useSpeechInput(jest.fn()))
    act(() => result.current.start())
    const started = latest
    unmount()
    expect(started?.aborted).toBeGreaterThan(0)
  })
})
