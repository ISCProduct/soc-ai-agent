// 生徒の傾向分析(Issue #1027)の型と表示ロジック。
// バックエンド `GET /api/admin/teacher/students/tendency-analysis` のレスポンスに対応する。

export interface CategoryScore {
  category: string
  score: number
}

export interface SuitedIndustry {
  industry_id: number
  industry_name: string
  score: number
}

// 低マッチのまま進行中の応募1件(Issue #1028)。
export interface LowMatchApplication {
  company_name: string
  match_score: number
  status: string
}

// 履歴書の対応要否(Issue #1030)。
export interface ResumeAttention {
  needs_attention: boolean
  has_document: boolean
  latest_score?: number | null
  reason?: string
}

export interface StudentTendency {
  user_id: number
  name: string
  email: string
  type_label: string
  top_categories: CategoryScore[] | null
  suited_industries: SuitedIndustry[] | null
  low_match_applications?: LowMatchApplication[] | null
  resume_status?: ResumeAttention | null
  // 最終活動日時（#1573）。チャット発言・応募更新・スコア更新のうち最新。記録が無ければ省略。
  last_activity_at?: string | null
  data_available: boolean
}

export interface StudentTendencyResponse {
  students: StudentTendency[]
  total: number
  limit: number
  offset: number
}

// スコア未計測の生徒に表示するラベル。タイプ名の代わりに必ずこれを出す。
export const NO_DATA_LABEL = '分析データ不足'

// 上位カテゴリ / 向いている業界は最大3件まで表示する
const TOP_N = 3

// data_available が false の生徒はスコアが未計測なので、タイプ名を出さない。
// (バックエンドが誤ってタイプ名を返しても画面には出さない)
export function displayTypeLabel(student: StudentTendency): string {
  if (!student.data_available) return NO_DATA_LABEL
  return student.type_label || NO_DATA_LABEL
}

export function displayCategories(student: StudentTendency): CategoryScore[] {
  if (!student.data_available || !student.top_categories) return []
  return student.top_categories.slice(0, TOP_N)
}

export function displayIndustries(student: StudentTendency): SuitedIndustry[] {
  if (!student.data_available || !student.suited_industries) return []
  return student.suited_industries.slice(0, TOP_N)
}

// スコアは整数ならそのまま、小数があれば小数第1位まで表示する
export function formatScore(score: number): string {
  return Number.isInteger(score) ? String(score) : score.toFixed(1)
}

// 低マッチのまま進行中の応募。無ければ空配列。
// バックエンドは該当なしのとき項目ごと省略する(omitempty)ので、null も空として扱う。
export function lowMatchApplications(student: StudentTendency): LowMatchApplication[] {
  return student.low_match_applications ?? []
}

// 「対応が必要な学生」とみなす未活動の日数（#1573）。教員入口と一覧の絞り込みで共用する。
export const INACTIVE_DAYS = 14

const DAY_MS = 24 * 60 * 60 * 1000

// 最終活動の表示。事実だけを書き、人物評価の言葉を使わない（#1573）。
export function lastActivityLabel(student: StudentTendency, now: Date): string {
  if (!student.last_activity_at) return '活動の記録なし'
  const at = new Date(student.last_activity_at)
  if (Number.isNaN(at.getTime())) return '活動の記録なし'
  const days = Math.floor((now.getTime() - at.getTime()) / DAY_MS)
  return days <= 0 ? '今日' : `${days}日前`
}

// 未活動の日数以上活動が無いか。記録なしも含める（バックエンドの絞り込みと同じ判定）。
export function isInactive(student: StudentTendency, now: Date, days = INACTIVE_DAYS): boolean {
  if (!student.last_activity_at) return true
  const at = new Date(student.last_activity_at).getTime()
  return Number.isNaN(at) || now.getTime() - at >= days * DAY_MS
}

// 履歴書要対応の表示文言。要対応でなければ空。
export function resumeAttentionLabel(student: StudentTendency): string {
  const status = student.resume_status
  if (!status?.needs_attention) return ''
  return status.reason || '要対応'
}
