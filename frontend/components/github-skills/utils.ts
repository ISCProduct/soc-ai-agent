import { AXES_ORDER } from './constants'
import type { LanguageStat, RepoSummary, SkillScore } from './types'

export type RadarPoint = { x: number; y: number }

export type RadarAxisLabel = RadarPoint & {
  label: string
  color: string
}

const DEFAULT_GRID_LEVELS = [0.25, 0.5, 0.75, 1.0] as const

/** 言語統計を使用率降順に並べ、上位8件に絞る */
export function sortLanguageStats(stats: LanguageStat[]): LanguageStat[] {
  return [...stats]
    .sort((a, b) => b.Percentage - a.Percentage)
    .slice(0, 8)
}

/** スコアが1件以上0より大きいか */
export function hasSkillScores(scores: SkillScore[]): boolean {
  return scores.some(s => s.Score > 0)
}

/** リポジトリ要約を fullName で置き換え、先頭に追加する */
export function upsertRepoSummary(
  summaries: RepoSummary[],
  newSummary: RepoSummary,
  fullName: string,
): RepoSummary[] {
  const filtered = summaries.filter(s => s.FullName !== fullName)
  return [newSummary, ...filtered]
}

export function radarAngleOf(index: number, axisCount: number): number {
  return (Math.PI * 2 * index) / axisCount - Math.PI / 2
}

export function radarPointOnAxis(
  index: number,
  ratio: number,
  center: number,
  radius: number,
  axisCount: number,
): RadarPoint {
  const angle = radarAngleOf(index, axisCount)
  return {
    x: center + radius * ratio * Math.cos(angle),
    y: center + radius * ratio * Math.sin(angle),
  }
}

export function buildRadarGridPolygons(
  center: number,
  radius: number,
  axisCount: number,
  levels: readonly number[] = DEFAULT_GRID_LEVELS,
): string[] {
  return levels.map(level =>
    Array.from({ length: axisCount }, (_, i) => {
      const point = radarPointOnAxis(i, level, center, radius, axisCount)
      return `${point.x},${point.y}`
    }).join(' '),
  )
}

export function buildRadarDataPolygon(
  scores: SkillScore[],
  center: number,
  radius: number,
  axisOrder: readonly string[] = AXES_ORDER,
): string {
  const scoreMap = Object.fromEntries(scores.map(s => [s.Category, s.Score]))
  const axisCount = axisOrder.length
  return axisOrder
    .map((cat, i) => {
      const score = scoreMap[cat] ?? 0
      const point = radarPointOnAxis(i, score / 100, center, radius, axisCount)
      return `${point.x},${point.y}`
    })
    .join(' ')
}

export function buildRadarAxisLabels(
  center: number,
  radius: number,
  labelOffset: number,
  categoryLabels: Record<string, string>,
  categoryColors: Record<string, string>,
  axisOrder: readonly string[] = AXES_ORDER,
): RadarAxisLabel[] {
  const axisCount = axisOrder.length
  return axisOrder.map((cat, i) => {
    const angle = radarAngleOf(i, axisCount)
    return {
      x: center + (radius + labelOffset) * Math.cos(angle),
      y: center + (radius + labelOffset) * Math.sin(angle),
      label: categoryLabels[cat] ?? cat,
      color: categoryColors[cat] ?? '#888',
    }
  })
}
