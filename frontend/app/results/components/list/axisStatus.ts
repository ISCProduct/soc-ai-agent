/**
 * 「いまの回答でわかっていること」4軸の判定（React 非依存の純関数）。
 */
import type { AnalysisScores } from '../../types'

/** この値以上なら「よく聞けています」 */
export const GOOD_AXIS_MIN = 80
/** この値以上、GOOD 未満なら「やや少ない」。これ未満は「まだ少ない」 */
export const SOME_AXIS_MIN = 40

export type AxisLevel = 'good' | 'some' | 'few'

export const AXIS_LEVEL_LABEL: Record<AxisLevel, string> = {
  good: 'よく聞けています',
  some: 'やや少ない',
  few: 'まだ少ない',
}

/** SP は列が狭いので、「よく聞けています」を短くする（Figma v4 SP）。 */
export const AXIS_LEVEL_LABEL_SHORT: Record<AxisLevel, string> = { ...AXIS_LEVEL_LABEL, good: 'よく聞けた' }

export interface AxisItem {
  key: keyof AnalysisScores
  label: string
  value: number
}

export function buildAxisItems(scores: AnalysisScores): AxisItem[] {
  return [
    { key: 'job', label: '職種', value: scores.job },
    { key: 'interest', label: '興味', value: scores.interest },
    { key: 'aptitude', label: '適性', value: scores.aptitude },
    { key: 'future', label: '将来', value: scores.future },
  ]
}

export function axisLevel(value: number): AxisLevel {
  if (value >= GOOD_AXIS_MIN) return 'good'
  if (value >= SOME_AXIS_MIN) return 'some'
  return 'few'
}

/** 「よく聞けています」に届いていない軸のうち、最も低いもの。全て届いていれば null（同値は並びの先頭を優先）。 */
export function findWeakestAxis(items: AxisItem[]): AxisItem | null {
  const weak = items.filter((i) => axisLevel(i.value) !== 'good')
  if (weak.length === 0) return null
  return weak.reduce((min, cur) => (cur.value < min.value ? cur : min))
}

/** 「次にやること」の文と、チャットへ戻る導線を出すか。 */
export function buildNextStep(weakest: AxisItem | null, isProvisional: boolean): { message: string; showChatAction: boolean } {
  if (isProvisional) {
    return { message: 'ここまでの回答で並べています。会話を続けると根拠が増え、順番も変わります。', showChatAction: true }
  }
  if (weakest) {
    return { message: `${weakest.label}の話がまだ少ないため、チャットで続けると順位が変わることがあります。`, showChatAction: true }
  }
  return { message: '回答はひととおり聞けています。気になる企業を「詳しく見る」で確認しましょう。', showChatAction: false }
}
