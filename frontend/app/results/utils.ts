/**
 * マッチング結果ページ向けの純粋ヘルパー（React state 非依存）。
 */
import type { CapitalRelation } from '@/lib/company-data'
import type {
  AnalysisScores,
  CategoryScores,
  Company,
  RecommendationsDiagnostics,
} from './types'

/** カテゴリスコアの表示ラベル（キー順は UI と一致させる） */
export const CATEGORY_SCORE_LABELS: Record<keyof CategoryScores, string> = {
  technical: '技術力',
  teamwork: 'チームワーク',
  leadership: 'リーダーシップ',
  creativity: '創造性',
  stability: '安定志向',
  growth: '成長意欲',
  work_life: 'ワークライフ',
  challenge: '挑戦意欲',
  detail: '緻密さ',
  communication: 'コミュニケーション',
}

/**
 * recommendations API の1件を画面用 Company に変換する。
 * 既存のフォールバック値・フィールド優先順位を維持する。
 */
export function mapRecommendationToCompany(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- API レスポンスの緩い形を許容
  rec: any,
  index: number,
): Company {
  return {
    id: String(rec.id || rec.ID || index + 1),
    matchId: rec.match_id || undefined,
    name: rec.category_name || rec.name || `企業 ${index + 1}`,
    industry: rec.industry || 'IT・ソフトウェア',
    location: rec.location || '東京都',
    employees: rec.employees || '未定',
    description: rec.reason || '詳細情報は準備中です',
    matchScore: rec.score || 0,
    tags: rec.tags || [],
    techStack: rec.tech_stack || [],
    categoryScores: normalizeCategoryScores(rec.category_scores),
    isFavorited: rec.is_favorited || false,
    isApplied: rec.is_applied || false,
    applicationId: rec.application_id || undefined,
  }
}

/** 部分オブジェクトの category_scores を 0 埋めする */
export function normalizeCategoryScores(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- API レスポンス
  raw: any,
): CategoryScores | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const keys = Object.keys(CATEGORY_SCORE_LABELS) as (keyof CategoryScores)[]
  const normalized = {} as CategoryScores
  for (const key of keys) {
    const value = Number(raw[key])
    normalized[key] = Number.isFinite(value) ? value : 0
  }
  return normalized
}

/**
 * 推薦が空のときのユーザー向けメッセージを組み立てる。
 */
export function buildEmptyRecommendationsMessage(
  reason?: string,
  diagnostics?: RecommendationsDiagnostics | null,
): string {
  let message = 'データがありません。診断を完了してから数秒待ち、ページを更新してください。'
  if (reason === 'insufficient_user_scores') {
    message = '判定結果を出すための根拠が不足しています。チャットで質問に回答してください。'
  } else if (reason === 'insufficient_company_data') {
    message = '現在、公開済みの企業データがありません。管理者が企業情報を公開するまでお待ちください。'
  }
  if (diagnostics) {
    // 内部指標はエンドユーザー向け文言に混在させず、調査用にconsoleへ出力するのみに留める
    console.debug('[results] empty recommendations diagnostics', diagnostics)
  }
  return message
}

/**
 * カテゴリスコアを降順ソートし、上位 N 件を返す。
 */
export function getTopCategoryScores(
  scores: CategoryScores,
  limit = 3,
): { label: string; score: number }[] {
  return (Object.keys(CATEGORY_SCORE_LABELS) as (keyof CategoryScores)[])
    .map((key) => ({ label: CATEGORY_SCORE_LABELS[key], score: Number(scores[key]) || 0 }))
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
}

/**
 * analysis API の scores を画面表示用（百分率）に変換する。
 */
export function mapAnalysisScores(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- API レスポンスの緩い形を許容
  scores: any,
): AnalysisScores {
  return {
    job: Math.round((scores.job_score || 0) * 100),
    interest: Math.round((scores.interest_score || 0) * 100),
    aptitude: Math.round((scores.aptitude_score || 0) * 100),
    future: Math.round((scores.future_score || 0) * 100),
  }
}

/**
 * 面接練習ページへのクエリ文字列を組み立てる。
 */
export function buildInterviewQuery(company: Pick<Company, 'id' | 'name' | 'industry'>): string {
  const params = new URLSearchParams({
    company_id: company.id,
    company_name: company.name,
    industry: company.industry,
  })
  return params.toString()
}

/**
 * ES添削・リライトページへのクエリ文字列を組み立てる（企業名プリフィル用）。
 */
export function buildEsRewriteQuery(company: Pick<Company, 'name'>): string {
  const params = new URLSearchParams({ company_name: company.name })
  return params.toString()
}

/**
 * 関係図用の関連企業 ID 集合を算出する（純粋）。
 */
export function collectRelatedCompanyIds(
  companyId: number,
  type: 'capital' | 'business',
  relations: CapitalRelation[],
): Set<number> {
  const relatedIds = new Set([companyId])

  relations.forEach((rel) => {
    if (type === 'capital' && rel.relation_type.startsWith('capital')) {
      if (rel.parent_id === companyId || rel.child_id === companyId) {
        if (rel.parent_id) relatedIds.add(rel.parent_id)
        if (rel.child_id) relatedIds.add(rel.child_id)
      }
    } else if (type === 'business' && rel.relation_type.startsWith('business')) {
      if (rel.from_id === companyId || rel.to_id === companyId) {
        if (rel.from_id === companyId && rel.to_id) relatedIds.add(rel.to_id)
        if (rel.to_id === companyId && rel.from_id) relatedIds.add(rel.from_id)
      }
    }
  })

  return relatedIds
}

/**
 * 関係データから企業名を解決する（純粋）。
 */
export function resolveCompanyNameFromRelations(
  id: number,
  relations: CapitalRelation[],
): string {
  for (const rel of relations) {
    if (rel.parent?.id === id) return rel.parent.name
    if (rel.child?.id === id) return rel.child.name
    if (rel.from?.id === id) return rel.from.name
    if (rel.to?.id === id) return rel.to.name
  }
  return `企業 ${id}`
}
