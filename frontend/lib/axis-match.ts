import { authService } from '@/lib/auth'
import { CATEGORY_SCORE_LABELS } from '@/app/results/utils'
import type { CategoryScores } from '@/app/results/types'

/**
 * 10カテゴリのスコアと、企業要件との軸別のマッチ（SOCAIAGENT-611）。
 * 推薦の順位を変えるものではなく、自分のどの傾向がどの企業要件と合っているかを知るための表示。
 */

/** 10軸のキー。並びは推薦の category_scores と同じ */
export const AXIS_KEYS = Object.keys(CATEGORY_SCORE_LABELS) as (keyof CategoryScores)[]

/**
 * 軸の表示名。推薦の一覧の「噛み合った軸」と同じ辞書（CATEGORY_SCORE_LABELS）を使い、同じ行に同じ軸が
 * 別の名前で並ばないようにする。照合はキーで行う（表示名を変えても「未計測」にならないように）。
 */
export function axisLabel(key: string, fallback = key): string {
  return (CATEGORY_SCORE_LABELS as Record<string, string>)[key] ?? fallback
}

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
export type MyCategoryScore = { key: string; label: string; score: number | null }

/** 見られない推薦（他人・存在しない・企業のプロファイルが無い）。読み込みの失敗と区別する */
export class AxisBreakdownNotFoundError extends Error {
  constructor() {
    super('この企業の内訳は見られません')
    this.name = 'AxisBreakdownNotFoundError'
  }
}

export async function fetchAxisBreakdown(matchId: number): Promise<AxisBreakdown> {
  const res = await fetch(`/api/chat/matches/${encodeURIComponent(String(matchId))}/axes`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
  })
  if (res.status === 404) throw new AxisBreakdownNotFoundError()
  if (!res.ok) throw new Error(`axes ${res.status}`)
  return (await res.json()) as AxisBreakdown
}

type AxisScoreRow = { key?: unknown; label?: unknown; score?: unknown }

/** /api/chat/axis-scores の axes を、キーで10軸の並びにそろえる。無い軸・数値でない値は未計測（null） */
export function toMyCategoryScores(rows: AxisScoreRow[]): MyCategoryScore[] {
  const byKey = new Map<string, number>()
  for (const r of rows) {
    if (typeof r.key === 'string' && typeof r.score === 'number' && Number.isFinite(r.score)) byKey.set(r.key, r.score)
  }
  return AXIS_KEYS.map((key) => ({
    key,
    label: axisLabel(key),
    score: byKey.has(key) ? Math.round(byKey.get(key) as number) : null,
  }))
}

/**
 * 自分の10の傾向。軸ごとの内訳の「あなた」と同じ値（適性検査を合流させた値）を取る。
 * /api/chat/scores は合流前のチャット由来の値なので、同じ画面で並べると数値が食い違う。
 */
export async function fetchMyCategoryScores(sessionId: string): Promise<MyCategoryScore[]> {
  const res = await fetch(`/api/chat/axis-scores?session_id=${encodeURIComponent(sessionId)}`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
  })
  if (!res.ok) throw new Error(`scores ${res.status}`)
  const data = (await res.json()) as { axes?: unknown }
  return toMyCategoryScores(Array.isArray(data?.axes) ? (data.axes as AxisScoreRow[]) : [])
}
