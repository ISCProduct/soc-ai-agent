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

export interface StudentTendency {
  user_id: number
  name: string
  email: string
  type_label: string
  top_categories: CategoryScore[] | null
  suited_industries: SuitedIndustry[] | null
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
