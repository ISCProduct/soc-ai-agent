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
import { useInterviewClock } from '@/app/interview/hooks/useInterviewClock'
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
const turnResponse = (
  meta: Record<string, string | boolean | number | string[]>,
  options?: { emptyAudio?: boolean },
): FakeResponse => {
  const boundary = 'turnboundary'
  const audioPart = options?.emptyAudio ? '' : 'AUDIO'
  const body =
    `--${boundary}\r\nContent-Type: application/json\r\n\r\n${JSON.stringify(meta)}\r\n` +
    `--${boundary}\r\nContent-Type: audio/mpeg\r\n\r\n${audioPart}\r\n` +
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

/**
 * 時計だけ進め、タイマーは鳴らさない。面接のタイマーは毎回 Date.now から経過を計算するので、
 * 30分ぶん（1800回）鳴らさなくても結果は同じ。全部鳴らすと1件2秒かかり、負荷が高いと5秒の上限を超える。
 */
const skipClock = (ms: number) => jest.setSystemTime(Date.now() + ms)

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
      if (url.includes('/user/weight-scores')) return jsonResponse({ weight_scores: [] })
      if (url.includes('/interviews') && url.endsWith('/start')) return jsonResponse({ id: 1, status: 'in_progress' })
      if (url.endsWith('/interviews')) return jsonResponse({ id: 1, status: 'created' })
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

  const isDetail = (u: string) => /\/interviews\/1(\?|$)/.test(u)

  it('参加すると作成→開始→/start-turn の順に呼び、冒頭の質問を書き起こしに出す', async () => {
    const media = fakeMedia()
    const { result, unmount } = renderSession(media)

    await act(async () => { await result.current.handleJoin() })

    const order = calls.filter(u => !u.includes('/api/auth/session'))
    expect(order[0]).toMatch(/\/interviews$/)
    expect(order[1]).toMatch(/\/interviews\/1\/start$/)
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
    expect(calls.some(u => u.endsWith('/interviews'))).toBe(false)

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

  it('空音声のあと音声が復旧したらバナーは消え、文字回答欄は残る（#1831）', async () => {
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })

    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({ user_text: '回答', ai_text: '次へ' }, { emptyAudio: true })
    }
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    await flush()
    await flush()

    expect(result.current.textContinueMode).toBe(true)
    expect(result.current.voiceDegradeText).toMatch(/再生できません/)

    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({ user_text: '2つ目', ai_text: '続けます' })
    }
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    await flush()
    await flush()

    expect(result.current.textContinueMode).toBe(true)
    expect(result.current.voiceDegradeText).toBeNull()

    unmount()
  })

  it('stt_failed なら文字回答欄に入り、sendTurnText は answer_text を送る（#1831）', async () => {
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })

    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({
        user_text: '（聞き取れませんでした）',
        ai_text: 'もう一度お願いします',
        stt_failed: true,
      })
    }
    act(() => { result.current.startRecording() })
    act(() => { result.current.stopRecording() })
    await flush()
    await flush()

    expect(result.current.textContinueMode).toBe(true)
    expect(result.current.voiceDegradeText).toMatch(/聞き取れません/)

    route = (url, init) => {
      if (!url.endsWith('/turn')) return undefined
      turnBodies.push(init?.body as FormData)
      return turnResponse({ user_text: 'チームで開発しました', ai_text: '具体的には？' })
    }
    await act(async () => { await result.current.sendTurnText('チームで開発しました') })
    await flush()

    const textTurn = turnBodies[1]
    expect(textTurn.get('answer_text')).toBe('チームで開発しました')
    expect(textTurn.get('audio')).toBeNull()
    expect(result.current.voiceDegradeText).toBeNull()
    expect(result.current.textContinueMode).toBe(true)

    unmount()
  })

  it('終了するとスコア取得→終了API→ポーリングの順に進み、レポートが届いたら ready にする', async () => {
    route = (url) => {
      if (isDetail(url)) return jsonResponse({ session: { id: 1 }, report: { id: 9, summary: 'よかった' } })
      if (url.includes('/user/weight-scores')) {
        const n = calls.filter(u => u.includes('/user/weight-scores')).length
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

    const scoreIdx = calls.findIndex(u => u.includes('/user/weight-scores'))
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

      skipClock((28 * 60 - 6) * 1000)
      await act(async () => { await jest.advanceTimersByTimeAsync(1000) })
      expect(result.current.sessionWarningShown).toBe(true)
      expect(calls.some(u => u.endsWith('/finish'))).toBe(false)

      skipClock((2 * 60 - 1) * 1000)
      await act(async () => { await jest.advanceTimersByTimeAsync(1000) })
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

  const fakeTimersExceptDate = () => jest.useFakeTimers({
    doNotFake: ['nextTick', 'queueMicrotask', 'setImmediate', 'performance', 'requestAnimationFrame', 'cancelAnimationFrame'],
  })

  it('/turn には録音開始時ではなく送信時点の残り時間・経過・質問総数を送る (#2573)', async () => {
    // 冒頭の質問の応答を止めておき、録音中に届かせて質問総数を変える
    let releaseStartTurn: () => void = () => {}
    route = (url) => {
      if (!url.includes('/start-turn')) return undefined
      return new Promise<FakeResponse>(resolve => {
        releaseStartTurn = () => resolve(turnResponse({ ai_text: '自己紹介をお願いします', total_questions: 5 }))
      })
    }
    fakeTimersExceptDate()
    try {
      const { result, unmount } = renderSession(fakeMedia())
      await act(async () => {
        void result.current.handleJoin()
        await jest.advanceTimersByTimeAsync(0)
      })
      expect(calls.some(u => u.includes('/start-turn'))).toBe(true)
      await act(async () => { await jest.advanceTimersByTimeAsync(10_000) })

      // 10秒時点で録音を始め、その間に冒頭の質問(総数5)が届き、さらに20秒たってから送る
      act(() => { result.current.startRecording() })
      await act(async () => {
        releaseStartTurn()
        await jest.advanceTimersByTimeAsync(20_000)
      })
      expect(result.current.totalQuestionCount).toBe(5)
      act(() => { result.current.stopRecording() })
      await act(async () => { await jest.advanceTimersByTimeAsync(0) })

      const body = turnBodies[0]
      expect(body.get('remaining_seconds')).toBe(String(30 * 60 - 30))
      expect(body.get('question_elapsed_seconds')).toBe('30')
      expect(body.get('total_questions')).toBe('5')
      expect(body.get('question_index')).toBe('1')

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('時間切れの後片付けが録画の取得を待つ間もタイマーで終了処理を重ねない (#2573)', async () => {
    let releaseVideo: () => void = () => {}
    const media = fakeMedia()
    ;(media.stopAndCollectVideoBlob as jest.Mock).mockImplementation(
      () => new Promise<null>(resolve => { releaseVideo = () => resolve(null) }),
    )
    fakeTimersExceptDate()
    try {
      const { result, unmount } = renderSession(media)
      await act(async () => { await result.current.handleJoin() })

      // 時間切れ後、録画の取得が返るまでにタイマーが何度も鳴る
      skipClock(30 * 60 * 1000)
      await act(async () => { await jest.advanceTimersByTimeAsync(5000) })
      await act(async () => {
        releaseVideo()
        await jest.advanceTimersByTimeAsync(0)
      })
      await act(async () => { await jest.advanceTimersByTimeAsync(0) })

      expect(media.stopAndCollectVideoBlob).toHaveBeenCalledTimes(1)
      expect(calls.filter(u => u.endsWith('/finish'))).toHaveLength(1)
      expect(result.current.errorMessage).toBe(FORCED_STOP_MESSAGE)

      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('終了後に再び参加すれば、もう一度終了できる (#2573)', async () => {
    const { result, unmount } = renderSession(fakeMedia())
    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await act(async () => { await result.current.handleJoin() })
    await act(async () => { await result.current.handleStop() })
    await flush()

    expect(calls.filter(u => u.endsWith('/finish'))).toHaveLength(2)

    unmount()
  })

  it('残り2分の警告は1回だけ立てる (#2573)', async () => {
    // useInterviewClock が setState に true を渡した回数を数える（true を渡すのは警告だけ）
    // 名前空間 import は複製になりスパイが届かないので、フックと同じモジュール本体を差し替える
    const ReactModule = jest.requireActual<typeof import('react')>('react')
    const realUseState = ReactModule.useState
    let trueCalls = 0
    jest.spyOn(ReactModule, 'useState').mockImplementation(((initial: unknown) => {
      const [value, set] = realUseState(initial)
      const counted = (v: unknown) => { if (v === true) trueCalls++; set(v) }
      return [value, counted]
    }) as typeof ReactModule.useState)
    fakeTimersExceptDate()
    try {
      const handleStop = jest.fn().mockResolvedValue(undefined)
      const { result, unmount } = renderHook(() => useInterviewClock({ selectedPosition, handleStop }))
      act(() => { result.current.startTimer() })
      // 残り2分の手前まで時計を飛ばし、そこから境目をまたいで10回鳴らす
      skipClock((28 * 60 - 5) * 1000)
      await act(async () => { await jest.advanceTimersByTimeAsync(15_000) })

      expect(result.current.sessionWarningShown).toBe(true)
      expect(trueCalls).toBe(1)

      if (result.current.timerRef.current) clearInterval(result.current.timerRef.current)
      unmount()
    } finally {
      jest.useRealTimers()
    }
  })

  it('アンマウントでレポートのポーリングを止める', async () => {
    route = (url) => (isDetail(url) ? jsonResponse({ session: { id: 1 }, report: null }) : undefined)
    fakeTimersExceptDate()
    try {
      const { result, unmount } = renderSession(fakeMedia())
      await act(async () => { await result.current.handleJoin() })
      await act(async () => {
        await result.current.handleStop()
        await jest.advanceTimersByTimeAsync(3000)
      })
      const detailCalls = calls.filter(isDetail).length
      expect(detailCalls).toBeGreaterThan(0)

      unmount()
      await act(async () => { await jest.advanceTimersByTimeAsync(9000) })
      expect(calls.filter(isDetail)).toHaveLength(detailCalls)
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
