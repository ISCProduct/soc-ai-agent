/**
 * @jest-environment jsdom
 */
/**
 * useInterviewSession の挙動を固定するテスト（#2569 のフック分割前に追加）。
 *
 * interviewStopTurnRace.test.ts が覆う「終了とターンの競合」以外の経路
 * （レポートポーリングの結果分岐・タイマー・メール送信・終了の再試行・質問番号の更新・
 * 音声再生・同意ダイアログ・参加失敗）を、フック本体を本番どおり動かして確かめる。
 * 差し替えるのはネットワーク（fetch）とブラウザAPIだけ。
 */
import { TextDecoder, TextEncoder } from 'util'
import { act, renderHook } from '@testing-library/react'

Object.assign(globalThis, { TextEncoder, TextDecoder })

import { useInterviewSession } from '@/app/interview/hooks/useInterviewSession'
import type { InterviewMedia } from '@/app/interview/hooks/useInterviewMedia'
import type { InterviewCompany, InterviewStatus, Position } from '@/app/interview/types'
import { FINISH_FAILED_MESSAGE, FORCED_STOP_MESSAGE } from '@/app/interview/finishOutcome'
import type { User } from '@/lib/auth'

type FakeResponse = {
  ok: boolean
  status: number
  headers: { get: (name: string) => string | null }
  text: () => Promise<string>
  json: () => Promise<unknown>
  arrayBuffer: () => Promise<ArrayBuffer>
}

const jsonResponse = (body: unknown, ok = true): FakeResponse => ({
  ok,
  status: ok ? 200 : 500,
  headers: { get: () => 'application/json' },
  text: async () => JSON.stringify(body),
  json: async () => body,
  arrayBuffer: async () => new ArrayBuffer(0),
})

/** 面接ターンの応答（multipart/mixed: JSONメタ + 音声） */
const turnResponse = (meta: Record<string, string | boolean | number>): FakeResponse => {
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

/** onstop を同期で呼ぶ MediaRecorder（競合は別テストで扱うので、ここでは隙間を作らない） */
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
    this.ondataavailable?.({ data: new Blob(['voice'], { type: 'audio/webm' }) })
    this.onstop?.()
  }
}

const tokenExpiringIn = (seconds: number): string =>
  `header.${btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + seconds }))}.sig`

const user: User = { user_id: 7, email: 'a@example.com' } as User
const interviewCompany = { id: 1, name: 'テスト株式会社' } as InterviewCompany
const selectedPosition = { title: '総合職', category: 'general', questions: 3 } as Position

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

type Route = (url: string, init?: RequestInit) => FakeResponse | Promise<FakeResponse> | undefined

describe('useInterviewSession の挙動', () => {
  const originalFetch = global.fetch
  let calls: string[]
  let turnBodies: FormData[]
  /** テストごとに差し替える応答。undefined を返すと既定の応答になる */
  let route: Route
  let setStatus: jest.Mock<void, [InterviewStatus]>

  beforeEach(() => {
    calls = []
    turnBodies = []
    route = () => undefined
    setStatus = jest.fn()
    window.sessionStorage.setItem('user_token', tokenExpiringIn(3600))
    window.localStorage.setItem('user_token', tokenExpiringIn(3600))
    Object.assign(globalThis, { MediaRecorder: FakeMediaRecorder, MediaStream: class {} })
    URL.createObjectURL = jest.fn(() => 'blob:fake')
    URL.revokeObjectURL = jest.fn()
    HTMLMediaElement.prototype.play = jest.fn().mockRejectedValue(new Error('jsdom: no audio'))
    HTMLMediaElement.prototype.load = jest.fn()
    jest.spyOn(console, 'error').mockImplementation(() => {})
    jest.spyOn(console, 'warn').mockImplementation(() => {})

    global.fetch = jest.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push(url)
      const custom = await route(url, init)
      if (custom) return custom
      if (url.includes('/api/auth/session')) return jsonResponse({ user_token: tokenExpiringIn(3600) })
      if (url.includes('/start-turn')) return turnResponse({ ai_text: '自己紹介をお願いします' })
      if (url.endsWith('/turn')) {
        turnBodies.push(init?.body as FormData)
        return turnResponse({ user_text: '回答', ai_text: '次の質問です' })
      }
      if (url.includes('/api/user/weight-scores')) return jsonResponse({ weight_scores: [] })
      if (url.includes('/api/interviews') && url.endsWith('/start')) return jsonResponse({ id: 1, status: 'in_progress' })
      if (url.endsWith('/api/interviews')) return jsonResponse({ id: 1, status: 'created' })
      return jsonResponse({})
    }) as unknown as typeof fetch
  })

  afterEach(() => {
    global.fetch = originalFetch
    window.sessionStorage.clear()
    window.localStorage.clear()
    jest.restoreAllMocks()
    delete (globalThis as { AudioContext?: unknown }).AudioContext
  })

  const renderSession = (media: InterviewMedia) =>
    renderHook(() => useInterviewSession({
      user,
      interviewCompany,
      companyQuestionHints: [],
      selectedPosition,
      media,
      status: 'connected',
      setStatus,
    }))

  const isDetail = (u: string) => /\/api\/interviews\/1(\?|$)/.test(u)

  it('参加すると作成→開始→/start-turn の順に呼び、冒頭の質問を書き起こしに出す', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })

    const order = calls.filter(u => !u.includes('/api/auth/session'))
    expect(order[0]).toMatch(/\/api\/interviews$/)
    expect(order[1]).toMatch(/\/api\/interviews\/1\/start$/)
    expect(order[2]).toMatch(/\/start-turn$/)
    expect(setStatus.mock.calls.map(c => c[0])).toEqual(['connecting', 'connected'])
    expect(result.current.session?.id).toBe(1)
    expect(result.current.utterances).toEqual([{ role: 'ai', text: '自己紹介をお願いします' }])
    expect(result.current.totalQuestionCount).toBe(3)
    expect(media.startVideoRecording).toHaveBeenCalledTimes(1)

    unmount()
  })

  it('参加に失敗したら error にしてメッセージを出し、メディアを止める', async () => {
    const media = fakeMedia()
    ;(media.ensureStream as jest.Mock).mockRejectedValue(new Error('NotAllowedError'))
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })

    expect(setStatus.mock.calls.map(c => c[0])).toEqual(['connecting', 'error'])
    expect(result.current.errorMessage).toContain('アクセスが拒否されました')
    expect(media.stopStream).toHaveBeenCalled()
    expect(calls.some(u => u.endsWith('/api/interviews'))).toBe(false)

    unmount()
  })

  it('同意前は参加せずに同意ダイアログを開く', async () => {
    const { result, unmount } = renderSession(fakeMedia())

    act(() => { result.current.handleJoinWithConsent() })
    await flush()

    expect(result.current.consentDialogOpen).toBe(true)
    expect(calls).toHaveLength(0)

    unmount()
  })

  it('ターンの応答で質問番号・総数・深掘り・カテゴリ・書き起こしを更新し、送信内容に反映する', async () => {
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })

    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({
        user_text: '1つ目の回答',
        ai_text: '2問目です',
        question_source: 'bank',
        total_questions: 5,
        is_deepening: true,
        question_category: 'motivation',
      })
    }
    act(() => { result.current.startRecording() })
    expect(result.current.isRecording).toBe(true)
    act(() => { result.current.stopRecording() })
    await flush()
    await flush()

    expect(result.current.currentQuestionIndex).toBe(2)
    expect(result.current.totalQuestionCount).toBe(5)
    expect(result.current.isDeepeningQuestion).toBe(true)
    expect(result.current.questionCategory).toBe('motivation')
    expect(result.current.turnPending).toBe(false)
    expect(result.current.isRecording).toBe(false)
    expect(result.current.utterances).toEqual([
      { role: 'ai', text: '自己紹介をお願いします' },
      { role: 'user', text: '1つ目の回答' },
      { role: 'ai', text: '2問目です' },
    ])
    const first = turnBodies[0]
    expect(first.get('turn_count')).toBe('1')
    expect(first.get('question_index')).toBe('1')
    expect(first.get('total_questions')).toBe('3')
    expect(first.get('company_name')).toBe('テスト株式会社')
    expect(first.get('question_duration_seconds')).toBe('180')
    expect((first.get('audio') as File).name).toBe('audio.webm')

    // 2ターン目は更新後の値で送る
    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({ user_text: '2つ目の回答', ai_text: '3問目です' })
    }
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    await flush()
    await flush()
    const second = turnBodies[1]
    expect(second.get('turn_count')).toBe('2')
    expect(second.get('question_index')).toBe('2')
    expect(second.get('total_questions')).toBe('5')
    expect(result.current.isDeepeningQuestion).toBe(false)
    expect(result.current.questionCategory).toBeNull()

    unmount()
  })

  it('ターンが失敗したらエラーを出し、応答待ちを解く', async () => {
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })

    route = (url) => (url.endsWith('/turn') ? jsonResponse({ error: '処理に失敗しました' }, false) : undefined)
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    expect(result.current.turnPending).toBe(true)
    await flush()
    await flush()

    expect(result.current.turnPending).toBe(false)
    expect(result.current.errorMessage).toBeTruthy()

    unmount()
  })

  it('終了するとスコア取得→終了API→ポーリングの順に進み、レポートが届いたら ready にする', async () => {
    route = (url) => {
      if (isDetail(url)) return jsonResponse({ session: { id: 1 }, report: { id: 9, summary: 'よかった' } })
      if (url.includes('/api/user/weight-scores')) {
        const n = calls.filter(u => u.includes('/api/user/weight-scores')).length
        return jsonResponse({ weight_scores: [{ category: n === 1 ? 'before' : 'after' }] })
      }
      return undefined
    }
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await flush()
    await flush()

    expect(setStatus).toHaveBeenLastCalledWith('finished')
    expect(result.current.errorMessage).toBeNull()
    expect(result.current.finishFailed).toBe(false)
    expect(result.current.reportStatus).toBe('ready')
    expect(result.current.report).toEqual({ id: 9, summary: 'よかった' })
    expect(result.current.reportStartedAt).not.toBeNull()
    expect(result.current.scoresBefore).toEqual([{ category: 'before' }])
    expect(result.current.scoresAfter).toEqual([{ category: 'after' }])

    const scoreIdx = calls.findIndex(u => u.includes('/api/user/weight-scores'))
    const finishIdx = calls.findIndex(u => u.endsWith('/finish'))
    const detailIdx = calls.findIndex(isDetail)
    expect(scoreIdx).toBeLessThan(finishIdx)
    expect(finishIdx).toBeLessThan(detailIdx)
    expect(calls.filter(isDetail)).toHaveLength(1)
    expect(calls[calls.length - 1]).toContain('session_id=interview-7')

    unmount()
  })

  it('生成中は進捗を出し、サーバーが失敗と返したら待たずに error にする', async () => {
    let stage = 'generating'
    route = (url) => {
      if (isDetail(url)) return jsonResponse({ session: { id: 1, report_stage: stage, report_percent: 40 }, report: null })
      return undefined
    }
    jest.useFakeTimers({
      doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'Date', 'requestAnimationFrame', 'cancelAnimationFrame'],
    })
    try {
      const { result, unmount } = renderSession(fakeMedia())
      await act(async () => { await result.current.handleJoin() })
      await act(async () => {
        await result.current.handleStop()
        await jest.advanceTimersByTimeAsync(0)
      })
      expect(result.current.reportStatus).toBe('pending')
      expect(result.current.reportProgress).not.toBeNull()
      expect(result.current.reportFailedByServer).toBe(false)

      stage = 'failed'
      await act(async () => { await jest.advanceTimersByTimeAsync(3000) })
      expect(result.current.reportFailedByServer).toBe(true)
      expect(result.current.reportStatus).toBe('error')

      // 止まったポーリングはもう叩かない
      const detailCalls = calls.filter(isDetail).length
      await act(async () => { await jest.advanceTimersByTimeAsync(9000) })
      expect(calls.filter(isDetail)).toHaveLength(detailCalls)

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('取得が3回続けて失敗したら error にする', async () => {
    route = (url) => (isDetail(url) ? jsonResponse({ error: 'x' }, false) : undefined)
    jest.useFakeTimers({
      doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'Date', 'requestAnimationFrame', 'cancelAnimationFrame'],
    })
    try {
      const { result, unmount } = renderSession(fakeMedia())
      await act(async () => { await result.current.handleJoin() })
      await act(async () => {
        await result.current.handleStop()
        await jest.advanceTimersByTimeAsync(0)
      })
      await act(async () => { await jest.advanceTimersByTimeAsync(3000) })
      expect(result.current.reportStatus).toBe('pending')
      await act(async () => { await jest.advanceTimersByTimeAsync(3000) })
      expect(result.current.reportStatus).toBe('error')
      expect(result.current.reportFailedByServer).toBe(false)

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('再生成に成功したらポーリングをやり直す', async () => {
    let hasReport = false
    route = (url) => {
      if (isDetail(url)) return jsonResponse({ session: { id: 1 }, report: hasReport ? { id: 2 } : null })
      return undefined
    }
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await flush()
    expect(result.current.reportStatus).toBe('pending')

    hasReport = true
    await act(async () => { await result.current.retryReportPolling() })
    await flush()
    await flush()

    const regenIdx = calls.findIndex(u => u.endsWith('/report/regenerate'))
    expect(regenIdx).toBeGreaterThan(-1)
    expect(calls.slice(regenIdx).some(isDetail)).toBe(true)
    expect(result.current.reportStatus).toBe('ready')
    expect(result.current.report).toEqual({ id: 2 })
    expect(result.current.reportRetryError).toBe('')

    unmount()
  })

  it('終了APIが失敗したら再試行UIを出し、retryFinish で成功したら消す', async () => {
    let finishOk = false
    route = (url) => (url.endsWith('/finish') && !finishOk ? jsonResponse({ error: 'x' }, false) : undefined)
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await flush()

    expect(result.current.finishFailed).toBe(true)
    expect(result.current.errorMessage).toBe(FINISH_FAILED_MESSAGE)
    // 終了APIが失敗してもポーリングは始める
    expect(calls.some(isDetail)).toBe(true)

    finishOk = true
    await act(async () => { await result.current.retryFinish() })
    expect(result.current.finishFailed).toBe(false)
    expect(result.current.errorMessage).toBeNull()

    unmount()
  })

  it('レポートのメール送信の成否を出す', async () => {
    let sendOk = true
    route = (url) => (url.endsWith('/send-report') ? jsonResponse({ message: 'ok' }, sendOk) : undefined)
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })

    await act(async () => { await result.current.sendReportEmail() })
    expect(result.current.emailSent).toBe(true)
    expect(result.current.emailError).toBe('')
    expect(result.current.emailSending).toBe(false)

    sendOk = false
    await act(async () => { await result.current.sendReportEmail() })
    expect(result.current.emailError).toBe('メールの送信に失敗しました。時間をおいて再度お試しください。')
    expect(result.current.emailSending).toBe(false)

    unmount()
  })

  it('経過時間を数え、残り2分で警告し、時間切れで強制終了する', async () => {
    jest.useFakeTimers({
      doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'requestAnimationFrame', 'cancelAnimationFrame'],
    })
    try {
      const { result, unmount } = renderSession(fakeMedia())
      expect(result.current.remainingSeconds).toBe(30 * 60)
      await act(async () => { await result.current.handleJoin() })

      await act(async () => { await jest.advanceTimersByTimeAsync(5000) })
      expect(result.current.elapsedSeconds).toBe(5)
      expect(result.current.remainingSeconds).toBe(30 * 60 - 5)
      expect(result.current.questionElapsedSeconds).toBe(5)
      expect(result.current.sessionWarningShown).toBe(false)

      await act(async () => { await jest.advanceTimersByTimeAsync((28 * 60 - 5) * 1000) })
      expect(result.current.sessionWarningShown).toBe(true)
      expect(calls.some(u => u.endsWith('/finish'))).toBe(false)

      await act(async () => { await jest.advanceTimersByTimeAsync(2 * 60 * 1000) })
      await act(async () => { await jest.advanceTimersByTimeAsync(0) })
      expect(calls.some(u => u.endsWith('/finish'))).toBe(true)
      expect(result.current.errorMessage).toBe(FORCED_STOP_MESSAGE)
      expect(setStatus).toHaveBeenLastCalledWith('finished')
      // 1問あたりの経過は上限（180秒）で止まる
      expect(result.current.questionElapsedSeconds).toBe(180)

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('AudioContext があれば応答音声の再生中だけ aiSpeaking を立てる', async () => {
    let ended: (() => void) | null = null
    HTMLMediaElement.prototype.play = jest.fn(function (this: HTMLMediaElement) {
      ended = () => this.onended?.(new Event('ended'))
      return Promise.resolve()
    })
    const close = jest.fn().mockResolvedValue(undefined)
    class FakeAudioContext {
      state = 'running'
      outputLatency = 0
      baseLatency = 0
      destination = {}
      resume = jest.fn().mockResolvedValue(undefined)
      close = close
      createMediaElementSource() { return { connect: jest.fn() } }
      createAnalyser() {
        return { fftSize: 0, smoothingTimeConstant: 0, connect: jest.fn(), getByteTimeDomainData: jest.fn() }
      }
    }
    Object.assign(globalThis, { AudioContext: FakeAudioContext })

    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => {
      void result.current.handleJoin()
      for (let i = 0; i < 20 && !ended; i++) await new Promise(resolve => setTimeout(resolve, 0))
    })
    expect(ended).not.toBeNull()
    expect(result.current.aiSpeaking).toBe(true)

    act(() => { ended?.() })
    expect(result.current.aiSpeaking).toBe(false)
    expect(result.current.aiLevel).toBe(0)

    // アンマウントで AudioContext を閉じ、メディアを止める
    unmount()
    expect(close).toHaveBeenCalled()
  })

  it('アンマウントでメディアを止める', () => {
    const media = fakeMedia()
    const { unmount } = renderSession(media)
    unmount()
    expect(media.stopStream).toHaveBeenCalledTimes(1)
  })
})
