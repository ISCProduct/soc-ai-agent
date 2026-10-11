/**
 * ThreeAvatar の結線判断（#1646）。
 *
 * 過去の不具合はどちらも lib 層ではなく結線側で起き、プレビュー用ハーネスも
 * 別経路で同じバグを共有していた。判断をここへ切り出し、ThreeAvatar と
 * テストが同じ関数を通るようにする。
 */
import type { AvatarCapabilities } from './avatar-capabilities'
import type { InterviewerState } from './avatar-motion'

/** 画面の状態から面接官の状態を決める。話している > 開始前 > 準備中 > 聞いている の順。 */
export function interviewerStateFor(s: { speaking: boolean; active: boolean; pending: boolean }): InterviewerState {
  if (s.speaking) return 'speaking'
  // 面接が始まる前に相槌を打つと、学生が何も話していないのにうなずく面接官になる。
  if (!s.active) return 'idle'
  if (s.pending) return 'thinking'
  return 'listening'
}

/** caps（AvatarMotion）が口を動かせるか。動かせるなら口は caps の持ち物。 */
export function capsOwnsMouth(caps: AvatarCapabilities | null): boolean {
  return caps !== null && (caps.mouthTargets.length > 0 || caps.jawBone !== null)
}

/**
 * 旧経路（モーフ・顎ボーン・ビセーム）のどれを動かすか。
 * caps が口を持つときは3つとも止める。同じモーフを2箇所から毎フレーム書くと
 * 値が競合して口が震え、顎ボーンも AvatarMotion の代替経路と二重になる。
 * ビセームは口モーフも顎も無いときだけの最後の手段。
 */
export function legacyMouthPaths(
  caps: AvatarCapabilities | null,
  has: { morph: boolean; jaw: boolean; viseme: boolean },
): { morph: boolean; jaw: boolean; viseme: boolean } {
  if (capsOwnsMouth(caps)) return { morph: false, jaw: false, viseme: false }
  return { morph: has.morph, jaw: has.jaw, viseme: !has.morph && !has.jaw && has.viseme }
}
