/**
 * @jest-environment jsdom
 */
/**
 * 「完了する」を押した時点で応答待ちのターンがあっても、その発話が保存されてから
 * 終了API（=レポート生成のキュー投入）を呼ぶことを固定する回帰テスト（#1476）。
 *
 * 保存チェーンのスナップショットだけを待つ実装では、/turn の応答が handleStop の後に
 * 届いた場合に user/ai の発話が finishSession の後に保存され、レポートが最後の
 * やり取り抜きで生成される。保存自体は成功するので警告も出ず、誰も気づけない。
 *
 * ここではフック本体（useInterviewSession）を本番どおり動かし、差し替えるのは
 * ネットワーク（fetch）とブラウザAPI（MediaRecorder / 音声再生）だけにしている。
 * 経路を迂回したモックでは、この順序バグは再現も検出もできない。
 */
import { TextDecoder, TextEncoder } from 'util'
import { act, renderHook } from '@testing-library/react'

Object.assign(globalThis, { TextEncoder, TextDecoder })

// authService は本物を使う。トークン更新(/api/auth/session)も面接ターンの一部なので、
// ここをモックで即解決にすると、更新が半開きで固まるケースを再現できない（#1501）。

import { useInterviewSession, REPORT_RETRY_FAILED_MESSAGE } from '@/app/interview/hooks/useInterviewSession'
import type { InterviewMedia } from '@/app/interview/hooks/useInterviewMedia'
import type { InterviewCompany, Position } from '@/app/interview/types'
import type { User } from '@/lib/auth'

type FakeResponse = {
  ok: boolean
  status: number
  headers: { get: (name: string) => string | null }
  text: () => Promise<string>
  json: () => Promise<unknown>
  arrayBuffer: () => Promise<ArrayBuffer>
}

const jsonResponse = (body: unknown): FakeResponse => ({
  ok: true,
  status: 200,
  headers: { get: () => 'application/json' },
  text: async () => JSON.stringify(body),
  json: async () => body,
  arrayBuffer: async () => new ArrayBuffer(0),
})

/** 面接ターンの応答（multipart/mixed: JSONメタ + 音声）を実際の形で組み立てる */
const turnResponse = (meta: Record<string, string>): FakeResponse => {
  const boundary = 'turnboundary'
  const body =
    `--${boundary}\r\nContent-Type: application/json\r\n\r\n${JSON.stringify(meta)}\r\n` +
    `--${boundary}\r\nContent-Type: audio/mpeg\r\n\r\nAUDIO\r\n` +
    `--${boundary}--\r\n`
  const bytes = new TextEncoder().encode(body)
  return {
    ok: true,
    status: 200,
    headers: { get: () => `multipart/mixed; boundary=${boundary}` },
    text: async () => body,
    json: async () => ({}),
    arrayBuffer: async () => bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer,
  }
}

/**
 * stop() の後に onstop が発火するまでの「隙間」を再現する MediaRecorder。
 *
 * 実物の stop() は state を即座に 'inactive' にする一方で、dataavailable / stop イベントは
 * 別タスクで発火する。onstop を同期で呼ぶフェイクだとこの隙間が消え、
 * 「送信ボタンを押した直後に面接が終了した」競合をテストで再現できない。
 */
const pendingRecorderStops: (() => void)[] = []
/** 保留中の onstop を発火させる（＝ブラウザが後から stop イベントを配送した状態） */
const deliverRecorderStops = () => {
  pendingRecorderStops.splice(0).forEach(fire => fire())
}

class FakeMediaRecorder {
  static isTypeSupported = () => true
  state: 'inactive' | 'recording' = 'inactive'
  ondataavailable: ((e: { data: Blob }) => void) | null = null
  onstop: (() => void) | null = null
  start() {
    this.state = 'recording'
  }
  stop() {
    this.state = 'inactive'
    pendingRecorderStops.push(() => {
      this.ondataavailable?.({ data: new Blob(['voice'], { type: 'audio/webm' }) })
      this.onstop?.()
    })
  }
}

/** exp だけ持つ JWT 風のトークン。authService は署名を見ず exp だけ読む */
const tokenExpiringIn = (seconds: number): string =>
  `header.${btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + seconds }))}.sig`

/** 保存済みトークンを差し替える。期限が近いと authService は /api/auth/session を叩く */
const storeUserToken = (token: string) => {
  window.sessionStorage.setItem('user_token', token)
  window.localStorage.setItem('user_token', token)
}

const user: User = { user_id: 7, email: 'a@example.com' } as User
const interviewCompany = { id: 1, name: 'テスト株式会社' } as InterviewCompany
const selectedPosition = { title: '総合職', category: 'general', questions: 3 } as Position

/** 面接画面が使うメディア機能のスタブ。録画は使わない。 */
function fakeMedia(): InterviewMedia {
  const track = { kind: 'audio', enabled: true, stop: jest.fn() }
  const stream = {
    getTracks: () => [track],
    getAudioTracks: () => [track],
    getVideoTracks: () => [],
  } as unknown as MediaStream
  return {
    streamRef: { current: stream },
    ensureStream: jest.fn().mockResolvedValue(stream),
    stopStream: jest.fn(),
    startVideoRecording: jest.fn(),
    stopAndCollectVideoBlob: jest.fn().mockResolvedValue(null),
    setMicEnabled: jest.fn(),
    setCameraEnabled: jest.fn(),
  } as unknown as InterviewMedia
}

const flush = () => act(async () => { await new Promise(resolve => setTimeout(resolve, 0)) })

/**
 * ヘッダーは返すが本文（音声）が届かないまま固まる応答。
 * 実際のブラウザは fetch に渡した signal を abort すると本文の読み込みも中断するので、
 * その挙動だけを再現する。中断されなければ永久に解決しない。
 */
const neverResolves = <T,>(signal?: AbortSignal | null): Promise<T> => new Promise<T>((_, reject) => {
  signal?.addEventListener('abort', () => reject(new Error('AbortError: The operation was aborted')))
})

const hangingBodyResponse = (signal?: AbortSignal | null): FakeResponse => {
  const never = <T,>(): Promise<T> => neverResolves<T>(signal)
  return {
    ok: true,
    status: 200,
    headers: { get: () => 'multipart/mixed; boundary=turnboundary' },
    text: never,
    json: never,
    arrayBuffer: never,
  }
}

describe('完了する と応答待ちターンの競合 (#1476)', () => {
  const originalFetch = global.fetch
  let calls: string[]
  let releaseTurn: ((res: FakeResponse) => void) | null
  /** true の間は /start-turn の応答を保留する（参加直後の応答待ち状態を再現する） */
  let holdStartTurn: boolean
  let releaseStartTurn: ((res: FakeResponse) => void) | null
  /** true の間は /turn がヘッダーだけ返して本文で固まる */
  let hangTurnBody: boolean
  /** true の間は /api/auth/session が応答せず固まる（middleware のトークン更新が半開き） */
  let hangAuthSession: boolean

  beforeEach(() => {
    calls = []
    releaseTurn = null
    holdStartTurn = false
    releaseStartTurn = null
    hangTurnBody = false
    hangAuthSession = false
    // 面接中は有効なトークンが入っている状態から始める
    storeUserToken(tokenExpiringIn(3600))
    pendingRecorderStops.length = 0
    Object.assign(globalThis, {
      MediaRecorder: FakeMediaRecorder,
      MediaStream: class {},
    })
    URL.createObjectURL = jest.fn(() => 'blob:fake')
    URL.revokeObjectURL = jest.fn()
    // jsdom は再生を実装していない。再生失敗として扱えば playAudioBlob は解決する。
    HTMLMediaElement.prototype.play = jest.fn().mockRejectedValue(new Error('jsdom: no audio'))
    HTMLMediaElement.prototype.load = jest.fn()
    jest.spyOn(console, 'error').mockImplementation(() => {})
    jest.spyOn(console, 'warn').mockImplementation(() => {})

    global.fetch = jest.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push(url)
      if (url.includes('/api/auth/session')) {
        if (hangAuthSession) return neverResolves<FakeResponse>(init?.signal)
        return jsonResponse({ user_token: tokenExpiringIn(3600) })
      }
      if (url.includes('/start-turn')) {
        if (!holdStartTurn) return turnResponse({ ai_text: '自己紹介をお願いします' })
        return new Promise<FakeResponse>(resolve => { releaseStartTurn = resolve })
      }
      if (url.includes('/turn')) {
        if (hangTurnBody) return hangingBodyResponse(init?.signal)
        // 応答をテスト側で任意のタイミングまで保留する（=ターン処理中の状態）
        return new Promise<FakeResponse>(resolve => { releaseTurn = resolve })
      }
      if (url.includes('/api/interviews') && url.includes('/start')) return jsonResponse({ id: 1, status: 'in_progress' })
      if (url.endsWith('/api/interviews')) return jsonResponse({ id: 1, status: 'created' })
      return jsonResponse({})
    }) as unknown as typeof fetch
  })

  afterEach(() => {
    global.fetch = originalFetch
    window.sessionStorage.clear()
    window.localStorage.clear()
    jest.restoreAllMocks()
  })

  const renderSession = (media: InterviewMedia) =>
    renderHook(() => useInterviewSession({
      user,
      interviewCompany,
      selectedPosition,
      media,
      status: 'connected',
      setStatus: jest.fn(),
    }))

  it('handleStop の後に届いたターンの発話も、finishSession より先に保存する', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })
    act(() => { result.current.startRecording() })
    // stop → onstop → sendTurn が走り、/turn の応答待ちになる
    act(() => { result.current.stopRecording() })
    act(() => { deliverRecorderStops() })
    await flush()
    expect(releaseTurn).not.toBeNull()

    await act(async () => {
      const stopping = result.current.handleStop()
      await new Promise(resolve => setTimeout(resolve, 0))
      // ターンの応答前に終了APIを呼んでしまうと、この発話はレポートに載らない
      expect(calls.some(u => u.includes('/finish'))).toBe(false)

      releaseTurn?.(turnResponse({ user_text: '最後の回答です', ai_text: 'ありがとうございました' }))
      await stopping
    })

    const saveIndexes = calls.map((u, i) => ({ u, i })).filter(c => c.u.includes('/utterances')).map(c => c.i)
    const finishIndex = calls.findIndex(u => u.includes('/finish'))
    expect(saveIndexes).toHaveLength(3) // start-turn の AI 発話 + 最後の user/ai
    expect(finishIndex).toBeGreaterThan(Math.max(...saveIndexes))

    unmount()
  })

  /**
   * handleJoin は /start-turn の完了前に状態を connected にするため、その応答待ち中でも
   * 録音ボタンが押せて /turn を始められる。実行中のターンを1つしか覚えない実装だと、
   * handleStop は後から始まった /turn しか待たず、遅れて返った /start-turn の質問が
   * finishSession の後に保存される。質問を欠いた書き起こしでレポートが確定してしまう。
   */
  it('/start-turn の応答待ち中に始めたターンがあっても、両方の発話を finishSession より先に保存する', async () => {
    holdStartTurn = true
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => {
      void result.current.handleJoin()
      await new Promise(resolve => setTimeout(resolve, 0))
    })
    expect(releaseStartTurn).not.toBeNull()

    // /start-turn の応答待ち中に、ユーザーが録音して回答を送る
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    act(() => { deliverRecorderStops() })
    await flush()
    expect(releaseTurn).not.toBeNull()

    // 後から始めた /turn だけ先に返る
    await act(async () => {
      releaseTurn?.(turnResponse({ user_text: '最後の回答です', ai_text: 'ありがとうございました' }))
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    await act(async () => {
      const stopping = result.current.handleStop()
      await new Promise(resolve => setTimeout(resolve, 0))
      // /start-turn がまだ応答待ちなので、ここで終了APIを呼ぶと質問が書き起こしから欠ける
      expect(calls.some(u => u.includes('/finish'))).toBe(false)

      releaseStartTurn?.(turnResponse({ ai_text: '自己紹介をお願いします' }))
      await stopping
    })

    const saveIndexes = calls.map((u, i) => ({ u, i })).filter(c => c.u.includes('/utterances')).map(c => c.i)
    const finishIndex = calls.findIndex(u => u.includes('/finish'))
    expect(saveIndexes).toHaveLength(3) // /turn の user/ai + 遅れて返った /start-turn の質問
    expect(finishIndex).toBeGreaterThan(Math.max(...saveIndexes))

    unmount()
  })

  /**
   * 「完了する」を /turn の応答待ち中に押した場合の音声再生(#1501)。
   * playAudioBlob が自分の入り口で音声世代を読む実装だと、cleanupConnection が先に世代を
   * 進めていても「更新後の世代」を読むため stale 判定を通過する。結果、既にレポート画面へ
   * 遷移しているのに面接官の音声が鳴り出す。/start-turn の応答が遅れた場合も同じ。
   */
  it('完了した後に届いたターンの音声は、/turn も /start-turn も再生しない', async () => {
    holdStartTurn = true
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    const play = HTMLMediaElement.prototype.play as jest.Mock

    await act(async () => {
      void result.current.handleJoin()
      await new Promise(resolve => setTimeout(resolve, 0))
    })
    expect(releaseStartTurn).not.toBeNull()

    // /start-turn の応答待ち中に、ユーザーが録音して回答を送る
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    act(() => { deliverRecorderStops() })
    await flush()
    expect(releaseTurn).not.toBeNull()
    // どちらも応答待ちなので、まだ再生は始まっていない
    expect(play).not.toHaveBeenCalled()

    // 応答待ちのまま「完了する」を押し、その後に両方の応答が届く
    await act(async () => {
      const stopping = result.current.handleStop()
      await new Promise(resolve => setTimeout(resolve, 0))
      releaseTurn?.(turnResponse({ user_text: '最後の回答です', ai_text: 'ありがとうございました' }))
      releaseStartTurn?.(turnResponse({ ai_text: '自己紹介をお願いします' }))
      await stopping
    })
    await flush()

    // 発話はレポートのために保存されるが、音声は鳴らさない
    expect(calls.filter(u => u.includes('/utterances'))).toHaveLength(3)
    expect(play).not.toHaveBeenCalled()

    unmount()
  })

  /**
   * ターンのタイムアウトはヘッダー受信までしか効かない実装だと、本文（音声）の受信中に
   * 半開きになったターンは永久に解決しない。handleStop がそれを待つので finishSession も
   * ポーリング開始も呼ばれず、画面は「生成中」のまま進まない。
   */
  it('ターンの本文受信が止まっても、タイムアウトで打ち切って finishSession へ進む', async () => {
    const advance = (ms: number) => act(async () => { await jest.advanceTimersByTimeAsync(ms) })
    try {
      hangTurnBody = true
      const media = fakeMedia()
      const { result, unmount } = renderSession(media)

      await act(async () => { await result.current.handleJoin() })
      act(() => { result.current.startRecording() })
      act(() => { result.current.stopRecording() })
      // タイムアウトのタイマーはこの後の sendTurn で張られるので、ここから偽タイマーにする。
      // Promise の解決に使うものまで偽にすると React の act が進まなくなるため、
      // 偽にするのは setTimeout / setInterval だけに絞る。
      jest.useFakeTimers({
        doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'Date', 'requestAnimationFrame', 'cancelAnimationFrame'],
      })
      act(() => { deliverRecorderStops() })
      await advance(0)
      // ヘッダーは返っているが本文（音声）が届かない状態
      expect(calls.some(u => u.endsWith('/turn'))).toBe(true)

      let stopped = false
      let stopping: Promise<void> = Promise.resolve()
      await act(async () => {
        stopping = result.current.handleStop().then(() => { stopped = true })
        await jest.advanceTimersByTimeAsync(0)
      })
      expect(stopped).toBe(false)
      expect(calls.some(u => u.includes('/finish'))).toBe(false)

      // ターンのタイムアウト（90秒）まで進める
      await advance(90_000)
      await act(async () => { await stopping })

      expect(stopped).toBe(true)
      expect(calls.some(u => u.includes('/finish'))).toBe(true)

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  /**
   * ターンは本体の通信を投げる前にトークン更新(/api/auth/session)を待つ（#1501）。
   * この更新に上限が無いと、middleware 側の更新通信が半開きになったとき
   * ターンのタイムアウト（90秒）はそもそも張られず、応答待ちのターンは永久に残る。
   * 画面だけ「レポートを生成中」へ進み、finishSession もポーリングも始まらない。
   */
  it('トークン更新が返らないターンがあっても、上限を超えたら終了処理へ進む', async () => {
    const advance = (ms: number) => act(async () => { await jest.advanceTimersByTimeAsync(ms) })
    try {
      const media = fakeMedia()
      const { result, unmount } = renderSession(media)

      await act(async () => { await result.current.handleJoin() })

      // 最終ターンの直前に有効期限が近づいた状態。ここから更新が半開きで固まる
      storeUserToken(tokenExpiringIn(10))
      hangAuthSession = true

      act(() => { result.current.startRecording() })
      act(() => { result.current.stopRecording() })
      jest.useFakeTimers({
        doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'Date', 'requestAnimationFrame', 'cancelAnimationFrame'],
      })
      act(() => { deliverRecorderStops() })
      await advance(0)
      // トークン更新で止まっているので、ターン本体はまだ投げられていない
      expect(calls.some(u => u.includes('/api/auth/session'))).toBe(true)
      expect(calls.some(u => u.endsWith('/turn'))).toBe(false)

      let stopped = false
      let stopping: Promise<void> = Promise.resolve()
      await act(async () => {
        stopping = result.current.handleStop().then(() => { stopped = true })
        await jest.advanceTimersByTimeAsync(0)
      })
      expect(stopped).toBe(false)
      expect(calls.some(u => u.includes('/finish'))).toBe(false)

      // 掴んだままの更新は返らないが、以降の通信は復帰した状態
      hangAuthSession = false
      // トークン更新の上限（15秒）まで進める。ターンの90秒より手前で打ち切れること
      await advance(15_000)
      await act(async () => { await stopping })

      expect(stopped).toBe(true)
      expect(calls.some(u => u.includes('/finish'))).toBe(true)
      // 失敗はユーザーに見せる（黙って「生成中」のままにしない）
      expect(result.current.errorMessage).toContain('タイムアウト')

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('録音中に完了した場合、その録音を新しいターンとして送らない', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })
    act(() => { result.current.startRecording() })

    await act(async () => { await result.current.handleStop() })
    act(() => { deliverRecorderStops() })
    await flush()

    // 終了後に /turn を投げると、その発話は finishSession より後に保存される
    expect(calls.filter(u => u.endsWith('/turn'))).toHaveLength(0)
    expect(calls.some(u => u.includes('/finish'))).toBe(true)

    unmount()
  })

  /**
   * stop() 済み・onstop 未発火の録音は state が 'inactive' で、録音中との区別が付かない。
   * ここを state で判定すると破棄フラグが立たず、sendTurn もまだ trackTurn を呼んでいないため
   * 終了処理は解決済みの古い Promise を待つだけになる。結果、finishSession の後に /turn が飛び、
   * 最後の回答がレポートから欠ける。
   */
  it('送信直後（onstop 発火前）に完了した場合も、その録音を新しいターンとして送らない', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })
    act(() => { result.current.startRecording() })
    // 送信ボタン。state は 'inactive' になるが onstop はまだ配送されていない
    act(() => { result.current.stopRecording() })
    expect(pendingRecorderStops).toHaveLength(1)

    await act(async () => { await result.current.handleStop() })
    // ここでブラウザが stop イベントを配送する
    act(() => { deliverRecorderStops() })
    await flush()

    expect(calls.filter(u => u.endsWith('/turn'))).toHaveLength(0)
    expect(calls.some(u => u.includes('/finish'))).toBe(true)

    unmount()
  })

  /**
   * 再生成APIが失敗した＝生成ジョブは入り直っていない。
   * それでもポーリングへ戻すと、ユーザーは失敗を知らされないまま「生成中」をさらに3分見せられ、
   * 再びタイムアウトする。失敗はその場でレポート画面に出す（#1476）。
   */
  it('レポート再生成のリクエストが失敗したら、ポーリングへ戻さずエラーを出す', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await flush()
    expect(result.current.reportRetryError).toBe('')

    const detailUrl = (u: string) => u.includes('/api/interviews/1?')
    const detailCallsBefore = calls.filter(detailUrl).length

    // 再試行ボタン。オフライン・401・500 などで再投入自体が失敗するケース。
    const okFetch = global.fetch
    global.fetch = jest.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/report/regenerate')) {
        calls.push(url)
        return { ok: false, status: 500, headers: { get: () => 'application/json' }, text: async () => '{"error":"internal"}' }
      }
      return (okFetch as unknown as (i: RequestInfo | URL, n?: RequestInit) => Promise<unknown>)(input, init)
    }) as unknown as typeof fetch

    await act(async () => { await result.current.retryReportPolling() })

    expect(calls.some(u => u.includes('/report/regenerate'))).toBe(true)
    expect(result.current.reportRetryError).toBe(REPORT_RETRY_FAILED_MESSAGE)
    // 再投入できていないのにポーリングを開始すると、また3分待たされる
    expect(calls.filter(detailUrl)).toHaveLength(detailCallsBefore)

    unmount()
  })
})
