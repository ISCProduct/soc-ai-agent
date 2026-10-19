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
  static availability: string = 'unavailable'
  static installResult = false
  static installCalls = 0
  static available(_o: unknown) {
    return Promise.resolve(FakeRecognition.availability)
  }
  static install(_o: unknown) {
    FakeRecognition.installCalls++
    return Promise.resolve(FakeRecognition.installResult)
  }

  // 実APIと同じくインスタンスのプロパティ。options オブジェクトではない。
  processLocally: boolean | undefined
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
    FakeRecognition.availability = 'unavailable'
    FakeRecognition.installResult = false
    FakeRecognition.installCalls = 0
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

  it('応答が返らないまま固まったら打ち切る', () => {
    // Chrome の音声認識はクラウドへ投げるため、通信が詰まると onend が返らない。
    // 実ブラウザで試したときは30秒待っても「聞き取り中」のままだった。
    // 放置するとマイクを掴み続けるので、時間で畳む。
    jest.useFakeTimers()
    try {
      installRecognition()
      const { result } = renderHook(() => useSpeechInput(jest.fn()))
      act(() => result.current.start())
      expect(result.current.listening).toBe(true)

      act(() => {
        jest.advanceTimersByTime(15_000)
      })

      expect(result.current.listening).toBe(false)
      expect(latest?.aborted).toBeGreaterThan(0)
      expect(result.current.error).toContain('キーボード')
    } finally {
      jest.useRealTimers()
    }
  })

  it('結果が返れば打ち切りは発火しない', () => {
    jest.useFakeTimers()
    try {
      installRecognition()
      const { result } = renderHook(() => useSpeechInput(jest.fn()))
      act(() => result.current.start())
      act(() => latest?.onend?.())
      act(() => {
        jest.advanceTimersByTime(30_000)
      })
      // 正常に終わったのにエラーを出さない。
      expect(result.current.error).toBeNull()
    } finally {
      jest.useRealTimers()
    }
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

describe('端末内での処理', () => {
  beforeEach(() => {
    latest = null
    FakeRecognition.availability = 'unavailable'
    FakeRecognition.installResult = false
    FakeRecognition.installCalls = 0
    removeRecognition()
  })
  afterEach(removeRecognition)

  it('画面を開いただけでは可否を調べない', async () => {
    // SpeechRecognition.available() は一部の Chromium でタブごと落ちる。
    // 読み込み時に呼ぶと全利用者がチャットを開けなくなる。
    FakeRecognition.availability = 'available'
    const spy = jest.spyOn(FakeRecognition, 'available')
    installRecognition()
    renderHook(() => useSpeechInput(jest.fn()))
    await act(async () => {})
    expect(spy).not.toHaveBeenCalled()
    spy.mockRestore()
  })

  it('押したときに調べ、使えるなら次から端末内で処理する', async () => {
    FakeRecognition.availability = 'available'
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))

    await act(async () => {
      result.current.start()
    })
    expect(result.current.local).toBe(true)

    // 2回目から processLocally が付く（1回目は待たずに始めている）
    await act(async () => {
      result.current.start()
    })
    expect(latest?.processLocally).toBe(true)
  })

  it('調べるのは1度だけ', async () => {
    FakeRecognition.availability = 'unavailable'
    const spy = jest.spyOn(FakeRecognition, 'available')
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    await act(async () => {
      result.current.start()
    })
    await act(async () => {
      result.current.start()
    })
    expect(spy).toHaveBeenCalledTimes(1)
    spy.mockRestore()
  })

  it('使えないときは processLocally を渡さない', async () => {
    // 使えないのに true を渡すと language-not-supported で落ちる。
    // Android / ChromeOS は端末内処理に未対応なので、ここを通る。
    FakeRecognition.availability = 'unavailable'
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    await act(async () => {
      result.current.start()
    })
    expect(result.current.local).toBe(false)
    expect(latest?.processLocally).toBeUndefined()
  })

  // abort() の後、古い認識器の end / error / result は非同期で届く。そのときには
  // 新しい認識が始まっているので、素通しすると新しい認識の打ち切りタイマーが消え、
  // マイクを握ったままボタンだけ戻る。停止直後にもう一度押すと再現する。
  it('古い認識器のイベントで新しい認識の状態を壊さない', async () => {
    installRecognition()
    const onText = jest.fn()
    const { result } = renderHook(() => useSpeechInput(onText))

    await act(async () => {
      result.current.start()
    })
    const old = latest as FakeRecognition

    await act(async () => {
      result.current.start()
    })
    const current = latest as FakeRecognition
    expect(current).not.toBe(old)
    expect(result.current.listening).toBe(true)

    // 遅れて届いた古い認識器の end。新しい認識は続いているので無視する。
    act(() => {
      old.onend?.()
    })
    expect(result.current.listening).toBe(true)

    // 古い認識器のエラーも、新しい認識のエラー表示にしない。
    act(() => {
      old.onerror?.({ error: 'aborted' })
    })
    expect(result.current.error).toBeNull()

    // 遅れて届いた古い認識結果も入力欄へ入れない。
    act(() => {
      old.onresult?.(finalResult('古い結果'))
    })
    expect(onText).not.toHaveBeenCalled()

    // 現在の認識器のイベントはこれまでどおり効く。
    act(() => {
      current.onresult?.(finalResult('新しい結果'))
    })
    expect(onText).toHaveBeenCalledWith('新しい結果')
  })

  it('言語パックは押したときに取りに行く', async () => {
    // 数十MBの通信が発生しうるので、画面を開いただけでは落とさない。
    FakeRecognition.availability = 'downloadable'
    FakeRecognition.installResult = true
    installRecognition()
    const { result } = renderHook(() => useSpeechInput(jest.fn()))

    expect(FakeRecognition.installCalls).toBe(0)
    await act(async () => {
      result.current.start()
    })
    expect(FakeRecognition.installCalls).toBe(1)
    expect(result.current.local).toBe(true)
  })

  it('端末内処理に対応していない実装でも動く', async () => {
    class Legacy extends FakeRecognition {}
    // @ts-expect-error 静的メソッドを消して古い実装を模す
    delete Legacy.available
    ;(window as unknown as Record<string, unknown>).SpeechRecognition = Legacy
    const { result } = renderHook(() => useSpeechInput(jest.fn()))
    await act(async () => {
      result.current.start()
    })
    expect(result.current.supported).toBe(true)
    expect(result.current.local).toBe(false)
  })
})
