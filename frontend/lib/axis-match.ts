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

/** 見られない推薦（他人・存在しない・企業のプロファイルが無い）。読み込みの失敗と区別する */
export class AxisBreakdownNotFoundError extends Error {
  constructor() {
    super('この企業の内訳は見られません')
    this.name = 'AxisBreakdownNotFoundError'
  }
}

export async function fetchAxisBreakdown(matchId: number, signal?: AbortSignal): Promise<AxisBreakdown> {
  const res = await fetch(`/api/chat/matches/${encodeURIComponent(String(matchId))}/axes`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
    signal,
  })
  if (res.status === 404) throw new AxisBreakdownNotFoundError()
  if (!res.ok) throw new Error(`axes ${res.status}`)
  return (await res.json()) as AxisBreakdown
}

type AxisScoreRow = { key?: unknown; label?: unknown; score?: unknown }

/** /api/chat/axis-scores の axes を10軸の並びにそろえる。無い軸・数値でない値は未計測（null） */
export function toMyCategoryScores(rows: AxisScoreRow[]): MyCategoryScore[] {
  const byLabel = new Map<string, number>()
  for (const r of rows) {
    if (typeof r.label === 'string' && typeof r.score === 'number' && Number.isFinite(r.score)) byLabel.set(r.label, r.score)
  }
  return AXES.map((a) => ({ label: a.label, score: byLabel.has(a.label) ? Math.round(byLabel.get(a.label) as number) : null }))
}

/**
 * 自分の10の傾向。軸ごとの内訳の「あなた」と同じ値（適性検査を合流させた値）を取る。
 * /api/chat/scores は合流前のチャット由来の値なので、同じ画面で並べると数値が食い違う。
 */
export async function fetchMyCategoryScores(sessionId: string, signal?: AbortSignal): Promise<MyCategoryScore[]> {
  const res = await fetch(`/api/chat/axis-scores?session_id=${encodeURIComponent(sessionId)}`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
    signal,
  })
  if (!res.ok) throw new Error(`scores ${res.status}`)
  const data = (await res.json()) as { axes?: unknown }
  return toMyCategoryScores(Array.isArray(data?.axes) ? (data.axes as AxisScoreRow[]) : [])
}
