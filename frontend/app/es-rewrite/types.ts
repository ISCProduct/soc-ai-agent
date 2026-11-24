import type { FormatFinding } from '@/components/feedback/FormatFindingsSection'

// ES添削・リライト画面で共有する型と定数（#2569 で page-content.tsx から分離）

export const QUESTION_TYPES = ['志望動機', '自己PR', '学チカ', 'ガクチカ', 'その他']

// RAG側 models.ES_TEXT_MAX_LENGTH と合わせる。
// 10,000字だった頃は7,300字を超えた入力が必ず 422 になっていた（#1564）
export const ES_TEXT_MAX_LENGTH = 6000

// RAG側 models.ES_CHAR_LIMIT_MIN / MAX と合わせる（設問の文字数上限 / #1523）
export const CHAR_LIMIT_MIN = 100
export const CHAR_LIMIT_MAX = 2000

export type EsMode = 'rewrite' | 'review'
export type CharLimitMode = 'within' | 'around'

export type StarBreakdown = {
  situation: string
  task: string
  action: string
  result: string
}

// 字数の結果(#1523 / #1606)。ES添削・ESリライトで同じ形を返す（生成経路が同じ / #1533）
export type CharLimitResult = {
  improved_text_length?: number
  // 指定字数に収まったか。上限未指定なら null
  char_limit_satisfied?: boolean | null
  // 改善文が目標レンジ（元の100〜130%）に入ったか。指定字数があるときは null
  improved_length_satisfied?: boolean | null
  // short は元の100%未満、long は130%超え。範囲内と指定字数があるときは null
  improved_length_issue?: 'short' | 'long' | null
}

export type RewriteResult = CharLimitResult & {
  rewritten_text: string
  star: StarBreakdown
}

export type ReviewResult = CharLimitResult & {
  specificity_score: number
  star_score: number
  company_fit_score: number | null
  length_balance_score: number
  feedback: string
  improved_text: string
  company_strategy?: string | null
  feedback_items?: ReviewFeedbackItem[]
  // 企業情報の取得元。"none" なら企業適合性を評価していない(#1524)
  company_context_source?: 'company_brief' | 'cache' | 'web_search' | 'none'
  // 同一設問の前回の添削(#1531)。初回・未ログイン・設問未入力のときは付かない。
  previous?: PreviousReview | null
  // ルールによる形式チェックの指摘(#1530)。「必ず直す」として AI の指摘とは分けて表示する
  format_findings?: FormatFinding[]
}

export type PreviousReview = {
  id: number
  created_at: string
  question: string
  question_type: string
  specificity_score: number
  star_score: number
  length_balance_score: number
  company_fit_score?: number | null
}

export type ReviewFeedbackItem = {
  score_key: 'specificity_score' | 'star_score' | 'company_fit_score' | 'length_balance_score'
  source_excerpt: string
  issue: string
  suggestion: string
}

export type AnchoredFeedbackItem = ReviewFeedbackItem & {
  index: number
  anchorId: string
  start: number
  end: number
}
