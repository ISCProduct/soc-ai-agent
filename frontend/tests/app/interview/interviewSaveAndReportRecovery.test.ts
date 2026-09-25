/**
 * 発話保存の再試行が二重保存にならないことを、実際の API 経路（interviewApi.saveUtterance →
 * fetch のリクエストボディ）で確かめる回帰テスト（#1476）。
 *
 * saveUtteranceWithRetry をモックした save で呼ぶだけのテストでは、
 * 「再試行のたびに別の ID を振ってしまう」実装バグを検出できない。
 * ここでは fetch を差し替えて、再試行で飛ぶリクエストの client_utterance_id が
 * 初回と同一であること（＝サーバー側の一意制約で弾ける形になっていること）を固定する。
 */
import { saveUtteranceWithRetry, newClientUtteranceId, flushThenFinish } from '@/app/interview/utteranceSave'

jest.mock('@/lib/auth/index', () => ({
  authService: {
    ensureFreshUserToken: jest.fn().mockResolvedValue(undefined),
    getUserFetchHeaders: () => ({}),
  },
}))

import { interviewApi } from '@/lib/interview/index'

type CapturedBody = { role: string; text: string; client_utterance_id: string }

const captureBodies = (fetchMock: jest.Mock): CapturedBody[] =>
  fetchMock.mock.calls.map(call => JSON.parse(String((call[1] as RequestInit).body)) as CapturedBody)

describe('発話保存の冪等キー (#1476)', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('再試行しても同じ client_utterance_id を送る（サーバー側で二重保存を弾ける）', async () => {
    // 1回目は「サーバーに届いたかどうか分からない」失敗、2回目で成功する。
    const fetchMock = jest.fn()
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValueOnce({ ok: true, text: async () => '' })
    global.fetch = fetchMock as unknown as typeof fetch

    const clientUtteranceId = newClientUtteranceId()
    const saved = await saveUtteranceWithRetry(
      () => interviewApi.saveUtterance(5, 7, 'user', 'こんにちは', clientUtteranceId),
      { sleep: async () => {} },
    )

    expect(saved).toBe(true)
    const bodies = captureBodies(fetchMock)
    expect(bodies).toHaveLength(2)
    expect(bodies[0].client_utterance_id).toBe(clientUtteranceId)
    expect(bodies[1].client_utterance_id).toBe(clientUtteranceId)
    expect(bodies[0].text).toBe(bodies[1].text)
  })

  it('別の発話には別の ID が付く（同一IDで送ると2件目が保存されなくなる）', () => {
    const ids = new Set(Array.from({ length: 200 }, () => newClientUtteranceId()))
    expect(ids.size).toBe(200)
  })

  it('crypto.randomUUID が無い環境（非 secure context）でも ID を発行できる', () => {
    const original = Object.getOwnPropertyDescriptor(globalThis, 'crypto')
    Object.defineProperty(globalThis, 'crypto', { value: undefined, configurable: true })
    try {
      const ids = new Set([newClientUtteranceId(), newClientUtteranceId()])
      expect(ids.size).toBe(2)
      for (const id of ids) expect(id.length).toBeGreaterThan(0)
    } finally {
      if (original) Object.defineProperty(globalThis, 'crypto', original)
    }
  })

  it('保存できなければ false を返し、握りつぶさない', async () => {
    const fetchMock = jest.fn().mockRejectedValue(new Error('network down'))
    global.fetch = fetchMock as unknown as typeof fetch
    jest.spyOn(console, 'error').mockImplementation(() => {})

    const clientUtteranceId = newClientUtteranceId()
    const saved = await saveUtteranceWithRetry(
      () => interviewApi.saveUtterance(5, 7, 'user', 'こんにちは', clientUtteranceId),
      { sleep: async () => {} },
    )

    expect(saved).toBe(false)
    // 再試行は全て同じ ID。ここが崩れると失敗後に遅れて届いた分が別発話として残る。
    const ids = new Set(captureBodies(fetchMock).map(b => b.client_utterance_id))
    expect(ids).toEqual(new Set([clientUtteranceId]))
  })
})

/**
 * 発話保存のキューが捌ける前にレポート生成を始めない（#1476）。
 *
 * finishSession はレポート生成をキューするため、保存が残ったまま呼ぶとバックエンドは
 * 不完全なログでレポートを正常に確定させ、あとから保存が成功しても作り直されない。
 * 「上限付きで待って、時間切れなら先へ進む」に戻すとこのテストが落ちる。
 */
describe('保存キューの完了前にレポート生成を始めない (#1476)', () => {
  it('保存が終わるまで finishSession を呼ばない', async () => {
    let releasePendingSaves: () => void = () => {}
    const pendingSaves = new Promise<void>(resolve => { releasePendingSaves = resolve })
    const finish = jest.fn().mockResolvedValue(undefined)

    const done = flushThenFinish(pendingSaves, finish)

    // 保存キューが捌けていない間は、どれだけタイマーが進んでも終了APIは呼ばれない
    jest.useFakeTimers()
    jest.advanceTimersByTime(60_000)
    jest.useRealTimers()
    await Promise.resolve()
    expect(finish).not.toHaveBeenCalled()

    releasePendingSaves()
    await done
    expect(finish).toHaveBeenCalledTimes(1)
  })

  it('終了APIの失敗は呼び出し側へ伝える（無言で成功扱いにしない）', async () => {
    const finish = jest.fn().mockRejectedValue(new Error('finish failed'))
    await expect(flushThenFinish(Promise.resolve(), finish)).rejects.toThrow('finish failed')
  })
})

/**
 * 未生成レポートを作り直せる経路（#1476）。
 * ポーリングを再開するだけでは、生成ジョブが失われている場合に永久に復旧しない。
 */
describe('レポート再生成API (#1476)', () => {
  const originalFetch = global.fetch

  afterEach(() => { global.fetch = originalFetch })

  it('再生成エンドポイントを POST で叩く', async () => {
    const fetchMock = jest.fn().mockResolvedValue({ ok: true, text: async () => '' })
    global.fetch = fetchMock as unknown as typeof fetch

    await interviewApi.regenerateReport(42, 7)

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('/api/interviews/42/report/regenerate')
    expect(init.method).toBe('POST')
    expect(JSON.parse(String(init.body))).toEqual({ user_id: 7 })
  })

  it('失敗はエラーとして返す（成功扱いにしない）', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      text: async () => JSON.stringify({ message: 'session is not finished' }),
    }) as unknown as typeof fetch

    await expect(interviewApi.regenerateReport(42, 7)).rejects.toThrow()
  })
})
