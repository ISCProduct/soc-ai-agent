import { computeScoreDeltas } from '@/components/ScoreUpdateBanner'
import type { WeightScore } from '@/components/ScoreUpdateBanner'

describe('computeScoreDeltas', () => {
  const before: WeightScore[] = [
    { weight_category: '論理性', score: 60 },
    { weight_category: '伝達力', score: 50 },
    { weight_category: '熱意', score: 70 },
    { weight_category: 'リーダーシップ', score: 40 },
  ]

  it('スコアが増加したカテゴリを検出する', () => {
    const after: WeightScore[] = [
      { weight_category: '論理性', score: 75 },
      { weight_category: '伝達力', score: 50 },
      { weight_category: '熱意', score: 70 },
      { weight_category: 'リーダーシップ', score: 40 },
    ]
    const deltas = computeScoreDeltas(before, after)
    expect(deltas).toHaveLength(1)
    expect(deltas[0]).toMatchObject({ category: '論理性', before: 60, after: 75, delta: 15 })
  })

  it('スコアが減少したカテゴリを検出する', () => {
    const after: WeightScore[] = [
      { weight_category: '論理性', score: 60 },
      { weight_category: '伝達力', score: 40 },
      { weight_category: '熱意', score: 70 },
      { weight_category: 'リーダーシップ', score: 40 },
    ]
    const deltas = computeScoreDeltas(before, after)
    expect(deltas).toHaveLength(1)
    expect(deltas[0]).toMatchObject({ category: '伝達力', before: 50, after: 40, delta: -10 })
  })

  it('変化がない場合は空配列を返す', () => {
    const after: WeightScore[] = [...before]
    const deltas = computeScoreDeltas(before, after)
    expect(deltas).toHaveLength(0)
  })

  it('複数カテゴリの変化を絶対値の大きい順で返す', () => {
    const after: WeightScore[] = [
      { weight_category: '論理性', score: 65 },    // +5
      { weight_category: '伝達力', score: 70 },    // +20
      { weight_category: '熱意', score: 60 },      // -10
      { weight_category: 'リーダーシップ', score: 40 },
    ]
    const deltas = computeScoreDeltas(before, after)
    expect(deltas).toHaveLength(3)
    expect(Math.abs(deltas[0].delta)).toBeGreaterThanOrEqual(Math.abs(deltas[1].delta))
    expect(Math.abs(deltas[1].delta)).toBeGreaterThanOrEqual(Math.abs(deltas[2].delta))
  })

  it('before に存在しないカテゴリは0からのデルタとして計算する', () => {
    const after: WeightScore[] = [
      { weight_category: '新カテゴリ', score: 50 },
    ]
    const deltas = computeScoreDeltas([], after)
    expect(deltas).toHaveLength(1)
    expect(deltas[0]).toMatchObject({ category: '新カテゴリ', before: 0, after: 50, delta: 50 })
  })

  it('after が空の場合は空配列を返す', () => {
    const deltas = computeScoreDeltas(before, [])
    expect(deltas).toHaveLength(0)
  })
})
