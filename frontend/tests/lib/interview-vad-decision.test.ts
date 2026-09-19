import {
  initialVadState,
  stepVad,
  updateNoiseFloor,
  startThreshold,
  speechThreshold,
  NOISE_FLOOR_MIN,
  NOISE_FLOOR_MAX,
  SPEECH_ABSOLUTE_MIN,
  CONFIRM_MS,
  SILENCE_MS,
  MIN_RECORDING_MS,
  type VadState,
} from '@/app/interview/lib/vadDecision'

/** 無音フレームを繰り返して暗騒音を収束させる。 */
function settleNoiseFloor(level: number, frames = 400): VadState {
  let state = initialVadState()
  let now = 0
  for (let i = 0; i < frames; i++) {
    now += 16
    state = stepVad(state, { rms: level, now, canStart: true }).state
  }
  return state
}

describe('暗騒音の追従', () => {
  it('下限を下回らない', () => {
    expect(updateNoiseFloor(NOISE_FLOOR_MIN, 0)).toBeGreaterThanOrEqual(NOISE_FLOOR_MIN)
  })

  it('上限を超えない（騒音で判定が壊れない）', () => {
    let v = NOISE_FLOOR_MIN
    for (let i = 0; i < 1000; i++) v = updateNoiseFloor(v, 10)
    expect(v).toBeLessThanOrEqual(NOISE_FLOOR_MAX)
  })

  it('静かになったときは速く追従する', () => {
    const loud = updateNoiseFloor(0.03, 0.001)
    const quiet = updateNoiseFloor(0.001, 0.03)
    // 下がる方向のほうが1フレームあたりの移動量が大きい
    expect(0.03 - loud).toBeGreaterThan(quiet - 0.001)
  })
})

describe('しきい値', () => {
  it('開始しきい値は発話しきい値より低い（頭切れ防止）', () => {
    for (const floor of [0.002, 0.005, 0.01, 0.02]) {
      expect(startThreshold(floor)).toBeLessThan(speechThreshold(floor))
    }
  })

  it('静かな環境でも発話しきい値は絶対下限を下回らない', () => {
    expect(speechThreshold(NOISE_FLOOR_MIN)).toBeGreaterThanOrEqual(SPEECH_ABSOLUTE_MIN)
  })
})

describe('録音の開始', () => {
  // 以前の固定しきい値 0.015 では小声が録音されなかった。
  it('静かな部屋の小声でも録音が始まる', () => {
    const state = settleNoiseFloor(0.001)
    const quietSpeech = 0.012 // 旧しきい値 0.015 未満
    const { action } = stepVad(state, { rms: quietSpeech, now: 10_000, canStart: true })
    expect(action).toBe('start')
  })

  // 逆に、暗騒音が高い部屋では暗騒音そのもので始まらないこと。
  it('騒がしい部屋の暗騒音では録音が始まらない', () => {
    const state = settleNoiseFloor(0.02)
    const { action } = stepVad(state, { rms: 0.02, now: 10_000, canStart: true })
    expect(action).toBe('none')
  })

  it('騒がしい部屋でも、暗騒音を明確に超えれば始まる', () => {
    const state = settleNoiseFloor(0.02)
    const { action } = stepVad(state, { rms: 0.2, now: 10_000, canStart: true })
    expect(action).toBe('start')
  })

  it('AI発話中などは開始しない', () => {
    const state = settleNoiseFloor(0.001)
    const { action } = stepVad(state, { rms: 0.5, now: 10_000, canStart: false })
    expect(action).toBe('none')
  })
})

describe('発話が確定しない録音の破棄', () => {
  it('物音だけで始まった録音は確認時間後に破棄される', () => {
    let state = settleNoiseFloor(0.001)
    const t0 = 10_000
    // 開始しきい値は超えるが発話しきい値には届かない音
    const faint = (startThreshold(state.noiseFloor) + speechThreshold(state.noiseFloor)) / 2
    const started = stepVad(state, { rms: faint, now: t0, canStart: true })
    expect(started.action).toBe('start')
    state = started.state
    expect(state.speechConfirmed).toBe(false)

    const after = stepVad(state, { rms: 0.0005, now: t0 + CONFIRM_MS, canStart: true })
    expect(after.action).toBe('discard')
    expect(after.state.recording).toBe(false)
  })

  it('破棄しても暗騒音の推定は引き継ぐ', () => {
    let state = settleNoiseFloor(0.01)
    const floorBefore = state.noiseFloor
    const t0 = 10_000
    const faint = (startThreshold(floorBefore) + speechThreshold(floorBefore)) / 2
    state = stepVad(state, { rms: faint, now: t0, canStart: true }).state
    const after = stepVad(state, { rms: 0.0005, now: t0 + CONFIRM_MS, canStart: true })
    expect(after.action).toBe('discard')
    // 較正結果が捨てられて下限に戻っていないこと。
    // 開始フレームで1回更新されるため厳密一致にはならない。
    expect(after.state.noiseFloor).toBeCloseTo(floorBefore, 2)
    expect(after.state.noiseFloor).toBeGreaterThan(NOISE_FLOOR_MIN * 2)
  })

  // 破棄・送信のあと、次のターンで較正し直さないこと。
  // やり直すと2ターン目以降の先頭が取りこぼされる。
  it('破棄後は較正なしですぐ開始できる', () => {
    let state = settleNoiseFloor(0.001)
    const t0 = 10_000
    const faint = (startThreshold(state.noiseFloor) + speechThreshold(state.noiseFloor)) / 2
    state = stepVad(state, { rms: faint, now: t0, canStart: true }).state
    state = stepVad(state, { rms: 0.0005, now: t0 + CONFIRM_MS, canStart: true }).state
    expect(state.recording).toBe(false)

    const next = stepVad(state, { rms: 0.5, now: t0 + CONFIRM_MS + 16, canStart: true })
    expect(next.action).toBe('start')
  })
})

describe('録音の停止', () => {
  function startSpeaking(noiseLevel = 0.001) {
    let state = settleNoiseFloor(noiseLevel)
    const t0 = 10_000
    state = stepVad(state, { rms: 0.5, now: t0, canStart: true }).state
    expect(state.recording).toBe(true)
    expect(state.speechConfirmed).toBe(true)
    return { state, t0 }
  }

  it('最短録音時間の前は無音でも止めない（息継ぎ）', () => {
    const { state, t0 } = startSpeaking()
    const r = stepVad(state, { rms: 0, now: t0 + MIN_RECORDING_MS - 1, canStart: true })
    expect(r.action).toBe('none')
  })

  it('無音が続いたら送信する', () => {
    let { state, t0 } = startSpeaking()
    let now = t0 + MIN_RECORDING_MS
    let action = stepVad(state, { rms: 0, now, canStart: true }).action
    state = stepVad(state, { rms: 0, now, canStart: true }).state
    expect(action).toBe('none')

    now += SILENCE_MS
    const r = stepVad(state, { rms: 0, now, canStart: true })
    expect(r.action).toBe('stop')
    expect(r.state.recording).toBe(false)
  })

  it('途中で話し直したら無音カウントがリセットされる', () => {
    let { state, t0 } = startSpeaking()
    let now = t0 + MIN_RECORDING_MS
    state = stepVad(state, { rms: 0, now, canStart: true }).state
    expect(state.silenceSince).not.toBeNull()

    now += 1000
    state = stepVad(state, { rms: 0.5, now, canStart: true }).state
    expect(state.silenceSince).toBeNull()

    // 話し直した後、無音がしきい値未満なら止まらない
    now += SILENCE_MS - 1
    expect(stepVad(state, { rms: 0, now, canStart: true }).action).toBe('none')
  })

  it('録音中は暗騒音を更新しない（自分の声を取り込まない）', () => {
    const { state, t0 } = startSpeaking()
    const before = state.noiseFloor
    const after = stepVad(state, { rms: 0.9, now: t0 + 100, canStart: true }).state
    expect(after.noiseFloor).toBe(before)
  })
})
