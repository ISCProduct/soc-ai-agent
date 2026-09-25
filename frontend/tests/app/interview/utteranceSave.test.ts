import {
  saveUtteranceWithRetry,
  UTTERANCE_SAVE_RETRY_DELAYS_MS,
  UTTERANCE_SAVE_FAILED_MESSAGE,
} from '@/app/interview/utteranceSave'

describe('saveUtteranceWithRetry', () => {
  // 遅延は実際には待たず、要求された待ち時間だけ記録する
  function fakeSleep() {
    const waited: number[] = []
    return { waited, sleep: async (ms: number) => { waited.push(ms) } }
  }

  it('1回目で成功したら再試行しない', async () => {
    const save = jest.fn().mockResolvedValue(undefined)
    const { waited, sleep } = fakeSleep()

    await expect(saveUtteranceWithRetry(save, { sleep })).resolves.toBe(true)
    expect(save).toHaveBeenCalledTimes(1)
    expect(waited).toEqual([])
  })

  it('一時的な失敗は再試行して成功を返す', async () => {
    const save = jest.fn()
      .mockRejectedValueOnce(new Error('network error'))
      .mockResolvedValue(undefined)
    const { waited, sleep } = fakeSleep()

    await expect(saveUtteranceWithRetry(save, { sleep })).resolves.toBe(true)
    expect(save).toHaveBeenCalledTimes(2)
    expect(waited).toEqual([UTTERANCE_SAVE_RETRY_DELAYS_MS[0]])
  })

  // #1476: ここが false を返さないと、保存できていないことが誰にも見えないまま面接が続く
  it('再試行しても失敗し続けたら false を返す（握りつぶさない）', async () => {
    const save = jest.fn().mockRejectedValue(new Error('401 unauthorized'))
    const { waited, sleep } = fakeSleep()

    await expect(saveUtteranceWithRetry(save, { sleep })).resolves.toBe(false)
    expect(save).toHaveBeenCalledTimes(UTTERANCE_SAVE_RETRY_DELAYS_MS.length + 1)
    expect(waited).toEqual([...UTTERANCE_SAVE_RETRY_DELAYS_MS])
  })

  it('再試行は短いバックオフで、面接を止めるほど待たない', () => {
    expect(UTTERANCE_SAVE_RETRY_DELAYS_MS.length).toBeGreaterThan(0)
    const total = UTTERANCE_SAVE_RETRY_DELAYS_MS.reduce((a, b) => a + b, 0)
    expect(total).toBeLessThanOrEqual(3000)
  })

  it('失敗メッセージは面接が続行できることと反映されないことの両方を伝える', () => {
    expect(UTTERANCE_SAVE_FAILED_MESSAGE).toContain('記録')
    expect(UTTERANCE_SAVE_FAILED_MESSAGE.length).toBeGreaterThan(0)
  })
})
