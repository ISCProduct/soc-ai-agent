/**
 * 「どこまで聞けているか」4軸の判定（React 非依存の純関数）。
 */
import type { AnalysisScores } from '../../types'

/** この値を下回る軸は「まだ聞けていない」として強調する */
export const LOW_AXIS_THRESHOLD = 50

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

export function isLowAxis(value: number): boolean {
  return value < LOW_AXIS_THRESHOLD
}

/** 50未満の軸のうち最も低いもの。無ければ null（同値は並びの先頭を優先）。 */
export function findLowestLowAxis(items: AxisItem[]): AxisItem | null {
  const lows = items.filter((i) => isLowAxis(i.value))
  if (lows.length === 0) return null
  return lows.reduce((min, cur) => (cur.value < min.value ? cur : min))
}

/** 棒の幅（%）。範囲外は 0〜100 に丸める。 */
export function barWidthPercent(value: number): number {
  return Math.max(0, Math.min(100, value))
}
