import { Live2DMotion, pulse } from '@/lib/interview/live2d-motion'

describe('pulse', () => {
  it('is 0 at both edges and peaks in the middle', () => {
    expect(pulse(0)).toBe(0)
    expect(pulse(1)).toBe(0)
    expect(pulse(0.5)).toBeCloseTo(1, 5)
  })
})

describe('Live2DMotion', () => {
  it('starts with no nod and no blink', () => {
    const motion = new Live2DMotion(() => 0.5)
    const frame = motion.update(0)
    expect(frame.nodProgress).toBeNull()
    // 初回呼び出しでまばたきタイマーを立てるだけで、即座には発火しない
    expect(frame.blinkProgress).toBeNull()
  })

  it('nods for NOD_DURATION_SEC (0.45s) after acknowledge() and then stops', () => {
    const motion = new Live2DMotion(() => 0.5)
    motion.acknowledge(10)
    expect(motion.update(10).nodProgress).toBeCloseTo(0, 5)
    expect(motion.update(10.2).nodProgress).toBeCloseTo(0.2 / 0.45, 5)
    expect(motion.update(10.46).nodProgress).toBeNull() // 境界を過ぎて終了
    expect(motion.update(10.5).nodProgress).toBeNull()
  })

  it('does not restack when acknowledge() is called again mid-nod (last call wins, matches Avatar2D)', () => {
    const motion = new Live2DMotion(() => 0.5)
    motion.acknowledge(0)
    motion.update(0.1)
    motion.acknowledge(0.2) // 相槌をもう1回。進行度が0.2基準にリセットされる
    const frame = motion.update(0.3)
    expect(frame.nodProgress).toBeCloseTo(0.1 / 0.45, 5)
  })

  it('blinks once within the configured interval using the injected rand', () => {
    // rand() が常に0を返すと間隔は最短(2.5s)に固定される
    const motion = new Live2DMotion(() => 0)
    expect(motion.update(0).blinkProgress).toBeNull()
    expect(motion.update(2.4).blinkProgress).toBeNull()
    expect(motion.update(2.5).blinkProgress).toBeCloseTo(0, 5)
    expect(motion.update(2.5 + 0.12).blinkProgress).toBeNull() // 1周期(0.12s)で終わる
  })

  it('schedules the next blink only after the previous one finishes', () => {
    const motion = new Live2DMotion(() => 0)
    motion.update(0) // blinkNext を 0+2.5=2.5 に確定させる
    motion.update(2.5) // 1回目のまばたき開始
    motion.update(2.63) // 終了(0.12s超)、次回を 2.63+2.5=5.13 にスケジュール
    expect(motion.update(5.12).blinkProgress).toBeNull()
    expect(motion.update(5.13).blinkProgress).toBeCloseTo(0, 5)
  })
})
