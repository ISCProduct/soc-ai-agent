import { authService } from '@/lib/auth'

/**
 * 10カテゴリのスコアと、企業要件との軸別のマッチ（SOCAIAGENT-611）。
 * 推薦の順位を変えるものではなく、自分のどの傾向がどの企業要件と合っているかを知るための表示。
 */

/** 10軸。並びは推薦の category_scores と同じ。label は user_weight_scores の weight_category と同じ名前 */
export const AXES = [
  { key: 'technical', label: '技術志向' },
  { key: 'teamwork', label: 'チームワーク志向' },
  { key: 'leadership', label: 'リーダーシップ志向' },
  { key: 'creativity', label: '創造性志向' },
  { key: 'stability', label: '安定志向' },
  { key: 'growth', label: '成長志向' },
  { key: 'work_life', label: 'ワークライフバランス' },
  { key: 'challenge', label: 'チャレンジ志向' },
  { key: 'detail', label: '細部志向' },
  { key: 'communication', label: 'コミュニケーション力' },
] as const

export type AxisRow = {
  key: string
  label: string
  /** まだ診断で測れていない軸は null */
  user_score: number | null
  company_weight: number
  match: number | null
}

export type AxisNote = { key: string; text: string }

export type AxisBreakdown = {
  match_id: number
  company_id: number
  company_name: string
  axes: AxisRow[]
  strengths: AxisNote[]
  gaps: AxisNote[]
  unmeasured: string[]
}

/** 自分の10カテゴリのスコア。測れていない軸は null */
export type MyCategoryScore = { label: string; score: number | null }

export async function fetchAxisBreakdown(matchId: number, signal?: AbortSignal): Promise<AxisBreakdown> {
  const res = await fetch(`/api/chat/matches/${encodeURIComponent(String(matchId))}/axes`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
    signal,
  })
  if (!res.ok) throw new Error(res.status === 404 ? 'この企業の内訳は見られません' : `axes ${res.status}`)
  return (await res.json()) as AxisBreakdown
}

type ScoreRow = { WeightCategory?: string; weight_category?: string; category?: string; Score?: number; score?: number }

/** 診断のスコアを10軸の並びにそろえる。API は Go のフィールド名（WeightCategory / Score）で返す */
export function toMyCategoryScores(rows: ScoreRow[]): MyCategoryScore[] {
  const byLabel = new Map<string, number>()
  for (const r of rows) {
    const label = r.WeightCategory ?? r.weight_category ?? r.category
    const score = r.Score ?? r.score
    if (label && typeof score === 'number') byLabel.set(label, score)
  }
  return AXES.map((a) => ({ label: a.label, score: byLabel.has(a.label) ? Math.round(byLabel.get(a.label) as number) : null }))
}

export async function fetchMyCategoryScores(sessionId: string, signal?: AbortSignal): Promise<MyCategoryScore[]> {
  const res = await fetch(`/api/chat/scores?session_id=${encodeURIComponent(sessionId)}`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
    signal,
  })
  if (!res.ok) throw new Error(`scores ${res.status}`)
  const data = (await res.json()) as unknown
  return toMyCategoryScores(Array.isArray(data) ? (data as ScoreRow[]) : [])
}
