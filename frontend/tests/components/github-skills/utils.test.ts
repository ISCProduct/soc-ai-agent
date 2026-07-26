import {
  buildRadarAxisLabels,
  buildRadarDataPolygon,
  buildRadarGridPolygons,
  hasSkillScores,
  radarAngleOf,
  radarPointOnAxis,
  sortLanguageStats,
  upsertRepoSummary,
} from '@/components/github-skills/utils'
import type { LanguageStat, RepoSummary, SkillScore } from '@/components/github-skills/types'

describe('sortLanguageStats', () => {
  it('使用率降順に並べ上位8件に絞る', () => {
    const stats: LanguageStat[] = [
      { Language: 'Go', Percentage: 10 },
      { Language: 'TypeScript', Percentage: 45 },
      { Language: 'Python', Percentage: 30 },
      { Language: 'Rust', Percentage: 5 },
      { Language: 'Java', Percentage: 3 },
      { Language: 'Ruby', Percentage: 2 },
      { Language: 'C', Percentage: 2 },
      { Language: 'Shell', Percentage: 1.5 },
      { Language: 'HTML', Percentage: 1 },
      { Language: 'CSS', Percentage: 0.5 },
    ]

    expect(sortLanguageStats(stats)).toEqual([
      { Language: 'TypeScript', Percentage: 45 },
      { Language: 'Python', Percentage: 30 },
      { Language: 'Go', Percentage: 10 },
      { Language: 'Rust', Percentage: 5 },
      { Language: 'Java', Percentage: 3 },
      { Language: 'Ruby', Percentage: 2 },
      { Language: 'C', Percentage: 2 },
      { Language: 'Shell', Percentage: 1.5 },
    ])
  })

  it('元配列を変更しない', () => {
    const stats: LanguageStat[] = [
      { Language: 'Go', Percentage: 10 },
      { Language: 'TypeScript', Percentage: 45 },
    ]
    sortLanguageStats(stats)
    expect(stats[0].Language).toBe('Go')
  })
})

describe('hasSkillScores', () => {
  it('Score が 0 より大きい項目があれば true', () => {
    const scores: SkillScore[] = [
      { ID: 1, UserID: 1, Category: 'Frontend', Score: 0 },
      { ID: 2, UserID: 1, Category: 'Backend', Score: 12.5 },
    ]
    expect(hasSkillScores(scores)).toBe(true)
  })

  it('全て 0 または空配列なら false', () => {
    expect(hasSkillScores([])).toBe(false)
    expect(hasSkillScores([
      { ID: 1, UserID: 1, Category: 'Frontend', Score: 0 },
    ])).toBe(false)
  })
})

describe('upsertRepoSummary', () => {
  const existing: RepoSummary[] = [
    {
      ID: 1,
      FullName: 'user/repo-a',
      SummaryText: 'A',
      TechReason: 'r',
      Challenge: 'c',
      Achievement: 'a',
    },
    {
      ID: 2,
      FullName: 'user/repo-b',
      SummaryText: 'B',
      TechReason: 'r',
      Challenge: 'c',
      Achievement: 'a',
    },
  ]

  it('同一 FullName を置き換え先頭に追加する', () => {
    const updated: RepoSummary = {
      ID: 3,
      FullName: 'user/repo-a',
      SummaryText: 'A updated',
      TechReason: 'r2',
      Challenge: 'c2',
      Achievement: 'a2',
    }

    expect(upsertRepoSummary(existing, updated, 'user/repo-a')).toEqual([
      updated,
      existing[1],
    ])
  })

  it('存在しない FullName なら先頭に追加する', () => {
    const created: RepoSummary = {
      ID: 4,
      FullName: 'user/repo-c',
      SummaryText: 'C',
      TechReason: 'r',
      Challenge: 'c',
      Achievement: 'a',
    }

    expect(upsertRepoSummary(existing, created, 'user/repo-c')).toEqual([
      created,
      ...existing,
    ])
  })
})

describe('radar chart geometry', () => {
  const center = 120
  const radius = 91.2
  const axisCount = 5

  it('radarAngleOf は先頭軸を上向き (-π/2) にする', () => {
    expect(radarAngleOf(0, axisCount)).toBeCloseTo(-Math.PI / 2)
  })

  it('radarPointOnAxis は ratio 0 で中心、1 で外周', () => {
    const inner = radarPointOnAxis(0, 0, center, radius, axisCount)
    expect(inner.x).toBeCloseTo(center)
    expect(inner.y).toBeCloseTo(center)

    const outer = radarPointOnAxis(0, 1, center, radius, axisCount)
    expect(outer.x).toBeCloseTo(center)
    expect(outer.y).toBeCloseTo(center - radius)
  })

  it('buildRadarGridPolygons はレベル数分の polygon 文字列を返す', () => {
    const polygons = buildRadarGridPolygons(center, radius, axisCount)
    expect(polygons).toHaveLength(4)
    polygons.forEach(pts => {
      expect(pts.split(' ')).toHaveLength(axisCount)
    })
  })

  it('buildRadarDataPolygon はスコア比率を反映する', () => {
    const scores: SkillScore[] = [
      { ID: 1, UserID: 1, Category: 'Frontend', Score: 100 },
      { ID: 2, UserID: 1, Category: 'Backend', Score: 0 },
      { ID: 3, UserID: 1, Category: 'Infrastructure', Score: 0 },
      { ID: 4, UserID: 1, Category: 'Database', Score: 0 },
      { ID: 5, UserID: 1, Category: 'Other', Score: 0 },
    ]

    const polygon = buildRadarDataPolygon(scores, center, radius)
    const firstPoint = polygon.split(' ')[0].split(',')
    const outer = radarPointOnAxis(0, 1, center, radius, axisCount)

    expect(Number(firstPoint[0])).toBeCloseTo(outer.x)
    expect(Number(firstPoint[1])).toBeCloseTo(outer.y)
  })

  it('buildRadarAxisLabels は軸ラベルと色を返す', () => {
    const labels = buildRadarAxisLabels(
      center,
      radius,
      22,
      { Frontend: 'フロントエンド' },
      { Frontend: '#4FC3F7' },
      ['Frontend'],
    )

    expect(labels).toHaveLength(1)
    expect(labels[0].label).toBe('フロントエンド')
    expect(labels[0].color).toBe('#4FC3F7')
  })
})
