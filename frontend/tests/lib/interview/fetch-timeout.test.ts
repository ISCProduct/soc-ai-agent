/** @jest-environment jsdom */

import { interviewApi } from '@/lib/interview'

/**
 * 面接APIの fetch が必ずタイムアウトすることを固定する（#1514）。
 *
 * 以前は `interviewFetch` の `timeoutMs` 省略時に素の `fetch` へ落ちていた。
 * 半開きの接続で固まると呼び出し側の await が解決しない。
 * 特に `finishSession` は useInterviewSession.handleStop が待ってから
 * レポートのポーリングを始めるため、ここで止まると「レポートを生成中」の
 * まま永久に止まり、失敗も表示されない。
 *
 * ローカルでは Backend が即答するので再現しない。会場Wi-Fiでしか出ない。
 * 「応答しないサーバー」を立てて、呼び出しが必ず返ることで固定する。
 */

jest.mock('@/lib/auth', () => ({
  authService: {
    ensureFreshUserToken: jest.fn().mockResolvedValue(undefined),
    getUserFetchHeaders: () => ({ 'X-User-Token': 't' }),
  },
}))

describe('面接APIのタイムアウト', () => {
  const original = global.fetch

  afterEach(() => {
    global.fetch = original
    jest.useRealTimers()
  })

  /** abort されるまで永久に返さない fetch。半開きの接続を模す。 */
  function hangingFetch() {
    return jest.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(Object.assign(new Error('aborted'), { name: 'AbortError' })),
        )
      })
    })
  }

  // タイムアウトを渡していなかった呼び出し。ここが素の fetch に落ちていた。
  const noTimeoutCalls: Array<[string, () => Promise<unknown>]> = [
    ['finishSession', () => interviewApi.finishSession(1, 1)],
    ['createSession', () => interviewApi.createSession(1, 'ja', 'female')],
    ['startSession', () => interviewApi.startSession(1, 1)],
    ['regenerateReport', () => interviewApi.regenerateReport(1, 1)],
    ['getReport', () => interviewApi.getReport(1, 1)],
  ]

  it.each(noTimeoutCalls)('%s は応答が無くても打ち切られる', async (_name, call) => {
    jest.useFakeTimers()
    global.fetch = hangingFetch() as unknown as typeof fetch

    const p = call()
    // 失敗を捕まえておく。打ち切られずに固まると、この await が解決しない。
    const settled = p.then(
      () => 'resolved',
      () => 'rejected',
    )

    await jest.advanceTimersByTimeAsync(60_000)
    await expect(settled).resolves.toBe('rejected')
  })

  it('AbortSignal 付きで呼ばれる（素の fetch へ落ちていない）', async () => {
    const spy = hangingFetch()
    global.fetch = spy as unknown as typeof fetch
    jest.useFakeTimers()

    const p = interviewApi.finishSession(1, 1).catch(() => undefined)
    await Promise.resolve()

    expect(spy).toHaveBeenCalled()
    const init = spy.mock.calls[0][1]
    expect(init?.signal).toBeDefined()

    await jest.advanceTimersByTimeAsync(60_000)
    await p
  })
})
