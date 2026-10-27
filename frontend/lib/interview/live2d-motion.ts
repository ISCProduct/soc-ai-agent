/**
 * Live2D面接官のうなずき・まばたきのタイミング。
 *
 * 3D(ThreeAvatar)時代にあった「聞いている/話している/考えている」の状態機械
 * (旧 avatar-motion.ts の AvatarMotion)は、2Dイラスト版(Avatar2D, SOCAIAGENT-2D)
 * への移行で削除され、もっと単純な作りに変わっている:
 *   - まばたきは状態と無関係に独立したタイマーで常時回る
 *   - うなずきは `acknowledgeSignal` が変わった瞬間だけ固定時間(450ms)動く
 * Live2Dでも同じ単純さに合わせる（Avatar2D.tsx の BLINK_INTERVAL_MS /
 * BLINK_DURATION_MS / NOD_DURATION_MS と同じ値）。
 *
 * ここでは「いつ・どれだけ」の進行度(0-1)だけを返す。実際のパラメータへの
 * 反映は呼び出し側（Live2DAvatar）が `nodAngleY` / `pulse` を使って行う。
 */

const BLINK_INTERVAL_SEC: [number, number] = [2.5, 6.0]
const BLINK_DURATION_SEC = 0.12
const NOD_DURATION_SEC = 0.45

/** 0→1→0 の滑らかな山。まばたきの開閉に使う。 */
export function pulse(progress: number): number {
  if (progress <= 0 || progress >= 1) return 0
  return Math.sin(progress * Math.PI)
}

export interface Live2DMotionFrame {
  /** うなずき中の進行度(0-1)。していなければ null。`nodAngleY(progress)` に渡す。 */
  nodProgress: number | null
  /** まばたき中の進行度(0-1)。していなければ null。`1 - pulse(progress)` が開き具合。 */
  blinkProgress: number | null
}

export class Live2DMotion {
  private readonly rand: () => number
  private nodStartedAt: number | null = null
  private blinkNext = 0
  private blinkStartedAt: number | null = null

  constructor(rand: () => number = Math.random) {
    this.rand = rand
  }

  private between([lo, hi]: [number, number]): number {
    return lo + this.rand() * (hi - lo)
  }

  /** 学生の発話の区切り（`acknowledgeSignal` の変化）。相槌を1回入れる。 */
  acknowledge(now: number): void {
    this.nodStartedAt = now
  }

  /** 1フレーム進める。now は秒（`performance.now()/1000` 等）。 */
  update(now: number): Live2DMotionFrame {
    return {
      nodProgress: this.updateNod(now),
      blinkProgress: this.updateBlink(now),
    }
  }

  private updateNod(now: number): number | null {
    if (this.nodStartedAt === null) return null
    const p = (now - this.nodStartedAt) / NOD_DURATION_SEC
    if (p >= 1) {
      this.nodStartedAt = null
      return null
    }
    return p
  }

  private updateBlink(now: number): number | null {
    if (this.blinkNext === 0) this.blinkNext = now + this.between(BLINK_INTERVAL_SEC)
    if (this.blinkStartedAt === null && now >= this.blinkNext) {
      this.blinkStartedAt = now
    }
    if (this.blinkStartedAt === null) return null
    const p = (now - this.blinkStartedAt) / BLINK_DURATION_SEC
    if (p >= 1) {
      this.blinkStartedAt = null
      this.blinkNext = now + this.between(BLINK_INTERVAL_SEC)
      return null
    }
    return p
  }
}
