/**
 * 面接官アバターの動き。うなずき・まばたき・視線・呼吸を面接の状態から作る。
 *
 * ここは「モデルが動ける」ことを前提にした層で、動けるかどうかの判定は
 * avatar-capabilities.ts が持つ。足りない部位は黙って飛ばす（面接は止めない）。
 *
 * 乱数は呼び出し側から渡せるようにしてテストを決定的にする。
 * Math.random を直接呼ぶと「たまに落ちるテスト」になる。
 */
import type { AvatarCapabilities } from './avatar-capabilities'

/** 面接官の状態。うなずきの出し方がこれで変わる。 */
export type InterviewerState =
  /** 学生が話している。相槌としてうなずく。 */
  | 'listening'
  /** 面接官が話している。口が動く。うなずきは出さない。 */
  | 'speaking'
  /** 次の質問を準備している。視線を少し外す。 */
  | 'thinking'
  /** 待機。呼吸とまばたきだけ。 */
  | 'idle'

export interface MotionConfig {
  /** うなずき1回の長さ(秒)。 */
  nodDurationSec: number
  /** うなずきの深さ(ラジアン)。 */
  nodDepthRad: number
  /** 聞いている間、うなずきを挟む間隔(秒)の下限と上限。 */
  nodIntervalSec: [number, number]
  /** まばたき1回の長さ(秒)。 */
  blinkDurationSec: number
  /** まばたきの間隔(秒)の下限と上限。 */
  blinkIntervalSec: [number, number]
  /** 口を開くときの追従率。大きいほど速い。 */
  mouthAttack: number
  /** 口を閉じるときの追従率。開くより遅くすると喋って見える。 */
  mouthRelease: number
}

export const DEFAULT_MOTION: MotionConfig = {
  // 日本語の相槌は浅く速い。深く長いと「居眠り」や「作り物」に見える。
  nodDurationSec: 0.45,
  nodDepthRad: 0.11,
  // 相槌が多すぎると不自然なので、1.8〜4.2秒に1回へ散らす。
  nodIntervalSec: [1.8, 4.2],
  blinkDurationSec: 0.12,
  blinkIntervalSec: [2.5, 6.0],
  // 立ち上がりを速く、戻りを遅く。同じ値にするとパクパクして不自然になる。
  mouthAttack: 0.22,
  mouthRelease: 0.14,
}

/** 0→1→0 の滑らかな山。うなずき・まばたきの1回ぶんの形。 */
export function pulse(progress: number): number {
  if (progress <= 0 || progress >= 1) return 0
  return Math.sin(progress * Math.PI)
}

/**
 * うなずきは「下げて戻す」なので、前半を速く後半をゆっくりにする。
 * 左右対称の sin だと機械的に見える。
 */
export function nodCurve(progress: number): number {
  if (progress <= 0 || progress >= 1) return 0
  // 前半 35% で最も深くなる
  const peak = 0.35
  const t = progress < peak
    ? progress / peak
    : 1 - (progress - peak) / (1 - peak)
  return Math.sin(t * Math.PI * 0.5)
}

interface Timer {
  /** 次に発火する時刻(秒)。 */
  next: number
  /** 発火中なら開始時刻、していなければ null。 */
  startedAt: number | null
}

export class AvatarMotion {
  private readonly cfg: MotionConfig
  private readonly rand: () => number
  private nod: Timer = { next: 0, startedAt: null }
  private blink: Timer = { next: 0, startedAt: null }
  /** 頭の静止姿勢。初回に覚えて、以降はここからの相対で動かす。 */
  private headRest: { x: number; y: number } | null = null
  private state: InterviewerState = 'idle'
  /** 視線のずらし量。thinking のときだけ効く。 */
  private gaze = 0
  /** 口の開き具合。急に変えるとパクパクするので補間する。 */
  private mouth = 0

  constructor(cfg: MotionConfig = DEFAULT_MOTION, rand: () => number = Math.random) {
    this.cfg = cfg
    this.rand = rand
  }

  private between([lo, hi]: [number, number]): number {
    return lo + this.rand() * (hi - lo)
  }

  setState(next: InterviewerState, now: number): void {
    if (next === this.state) return
    this.state = next
    // 聞く側に入った瞬間に1回うなずく。相槌は「話し終わってから」では遅い。
    if (next === 'listening') {
      this.nod.startedAt = now
      this.nod.next = now + this.cfg.nodDurationSec + this.between(this.cfg.nodIntervalSec)
    }
  }

  /** 学生の発話が切れた（文の区切り）。相槌を1回入れる。 */
  acknowledge(now: number): void {
    if (this.state !== 'listening') return
    if (this.nod.startedAt !== null) return // うなずき中なら重ねない
    this.nod.startedAt = now
    this.nod.next = now + this.cfg.nodDurationSec + this.between(this.cfg.nodIntervalSec)
  }

  /**
   * 1フレーム進める。caps に無い部位は触らない。
   * now は秒。mouthLevel は 0–1 の音声振幅。
   */
  update(caps: AvatarCapabilities, now: number, mouthLevel: number): void {
    this.updateBlink(caps, now)
    this.updateHead(caps, now)
    this.updateMouth(caps, mouthLevel)
  }

  private updateBlink(caps: AvatarCapabilities, now: number): void {
    if (caps.blinkTargets.length === 0) return
    if (this.blink.next === 0) this.blink.next = now + this.between(this.cfg.blinkIntervalSec)

    if (this.blink.startedAt === null && now >= this.blink.next) {
      this.blink.startedAt = now
    }
    let value = 0
    if (this.blink.startedAt !== null) {
      const p = (now - this.blink.startedAt) / this.cfg.blinkDurationSec
      if (p >= 1) {
        this.blink.startedAt = null
        this.blink.next = now + this.between(this.cfg.blinkIntervalSec)
      } else {
        value = pulse(p)
      }
    }
    caps.vrmExpression?.setBlink(value)
    for (const t of caps.blinkTargets) {
      if (t.mesh.morphTargetInfluences) t.mesh.morphTargetInfluences[t.index] = value
    }
  }

  private updateHead(caps: AvatarCapabilities, now: number): void {
    const head = caps.headBone
    if (!head) return
    if (!this.headRest) {
      this.headRest = { x: head.rotation.x, y: head.rotation.y }
    }

    // うなずき
    let nodAmount = 0
    if (this.state === 'listening') {
      if (this.nod.startedAt === null && now >= this.nod.next) {
        this.nod.startedAt = now
      }
      if (this.nod.startedAt !== null) {
        const p = (now - this.nod.startedAt) / this.cfg.nodDurationSec
        if (p >= 1) {
          this.nod.startedAt = null
          this.nod.next = now + this.between(this.cfg.nodIntervalSec)
        } else {
          nodAmount = nodCurve(p) * this.cfg.nodDepthRad
        }
      }
    } else {
      this.nod.startedAt = null
    }

    // 視線。考えているときだけ少し外す。ずっと正面を見続けると不自然。
    const gazeTarget = this.state === 'thinking' ? 0.18 : 0
    this.gaze += (gazeTarget - this.gaze) * 0.04

    // 呼吸。常に乗せる微動。止まっていると人形に見える。
    const breath = Math.sin(now * 0.8) * 0.012

    head.rotation.x = this.headRest.x + nodAmount + breath
    head.rotation.y = this.headRest.y + this.gaze
  }

  private updateMouth(caps: AvatarCapabilities, level: number): void {
    const clamped = Math.max(0, Math.min(1, level))
    const target = this.state === 'speaking' ? clamped : 0
    const rate = target > this.mouth ? this.cfg.mouthAttack : this.cfg.mouthRelease
    this.mouth += (target - this.mouth) * rate
    // 追従なので厳密には 0 に落ちない。閉じ切らないと口が半開きのままに見える。
    if (this.mouth < 0.01) this.mouth = 0
    const open = this.mouth
    caps.vrmExpression?.setMouthOpen(open)
    for (const t of caps.mouthTargets) {
      if (t.mesh.morphTargetInfluences) t.mesh.morphTargetInfluences[t.index] = open
    }
    // モーフが無いモデルは顎ボーンで代替する
    if (caps.mouthTargets.length === 0 && !caps.vrmExpression?.hasMouth && caps.jawBone) {
      caps.jawBone.rotation.x = open * 0.25
    }
  }
}
