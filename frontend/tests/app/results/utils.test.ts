/**
 * @jest-environment jsdom
 */
import type { CapitalRelation } from '@/lib/company-data'
import type { CategoryScores } from '@/app/results/types'
import {
  buildEmptyRecommendationsMessage,
  buildInterviewQuery,
  collectRelatedCompanyIds,
  getTopCategoryScores,
  mapAnalysisScores,
  mapRecommendationToCompany,
  resolveCompanyNameFromRelations,
} from '@/app/results/utils'

describe('mapRecommendationToCompany', () => {
  it('API フィールドを画面用 Company にマッピングする', () => {
    const company = mapRecommendationToCompany(
      {
        id: 10,
        match_id: 99,
        category_name: '株式会社テスト',
        industry: 'SIer',
        location: '大阪',
        employees: '100-500',
        reason: 'マッチ理由',
        score: 87,
        tags: ['成長'],
        tech_stack: ['Go'],
        category_scores: { technical: 80 },
        is_favorited: true,
        is_applied: true,
        application_id: 5,
      },
      0,
    )

    expect(company).toEqual({
      id: '10',
      matchId: 99,
      name: '株式会社テスト',
      industry: 'SIer',
      location: '大阪',
      employees: '100-500',
      description: 'マッチ理由',
      matchScore: 87,
      tags: ['成長'],
      techStack: ['Go'],
      categoryScores: expect.objectContaining({ technical: 80, teamwork: 0 }),
      isFavorited: true,
      isApplied: true,
      applicationId: 5,
    })
  })

  it('部分的な category_scores を 0 埋めする', () => {
    const company = mapRecommendationToCompany({ category_scores: { technical: 80 } }, 0)
    expect(company.categoryScores?.technical).toBe(80)
    expect(company.categoryScores?.teamwork).toBe(0)
    expect(company.categoryScores?.communication).toBe(0)
  })

  it('欠落フィールドにフォールバックを適用する', () => {
    const company = mapRecommendationToCompany({}, 2)

    expect(company.id).toBe('3')
    expect(company.name).toBe('企業 3')
    expect(company.industry).toBe('IT・ソフトウェア')
    expect(company.location).toBe('東京都')
    expect(company.employees).toBe('未定')
    expect(company.description).toBe('詳細情報は準備中です')
    expect(company.matchScore).toBe(0)
    expect(company.tags).toEqual([])
    expect(company.techStack).toEqual([])
    expect(company.isFavorited).toBe(false)
    expect(company.isApplied).toBe(false)
  })

  it('ID が大文字の場合も解決する', () => {
    expect(mapRecommendationToCompany({ ID: 42, name: 'A' }, 0).id).toBe('42')
  })
})

describe('buildEmptyRecommendationsMessage', () => {
  it('デフォルトメッセージを返す', () => {
    expect(buildEmptyRecommendationsMessage()).toContain('データがありません')
  })

  it('insufficient_user_scores の場合は根拠不足メッセージ', () => {
    expect(buildEmptyRecommendationsMessage('insufficient_user_scores')).toContain('根拠が不足')
  })

  it('insufficient_company_data の場合は企業データ不足メッセージ', () => {
    expect(buildEmptyRecommendationsMessage('insufficient_company_data')).toContain('公開済みの企業データがありません')
  })

  it('diagnostics を末尾に付与する', () => {
    const message = buildEmptyRecommendationsMessage('matching_results_empty', {
      user_score_count: 1,
      active_company_count: 2,
      weight_profile_count: 3,
    })
    expect(message).toContain('スコア数: 1')
    expect(message).toContain('企業数: 2')
    expect(message).toContain('プロファイル数: 3')
  })

  it('diagnostics 欠損フィールドは - で表示する', () => {
    const message = buildEmptyRecommendationsMessage('matching_results_empty', {
      user_score_count: 1,
    })
    expect(message).toContain('スコア数: 1')
    expect(message).toContain('企業数: -')
    expect(message).toContain('プロファイル数: -')
    expect(message).not.toContain('undefined')
  })
})

describe('getTopCategoryScores', () => {
  const scores: CategoryScores = {
    technical: 10,
    teamwork: 90,
    leadership: 50,
    creativity: 20,
    stability: 30,
    growth: 80,
    work_life: 40,
    challenge: 70,
    detail: 5,
    communication: 60,
  }

  it('スコア降順で上位 N 件を返す', () => {
    expect(getTopCategoryScores(scores, 3)).toEqual([
      { label: 'チームワーク', score: 90 },
      { label: '成長意欲', score: 80 },
      { label: '挑戦意欲', score: 70 },
    ])
  })

  it('limit 省略時は 3 件', () => {
    expect(getTopCategoryScores(scores)).toHaveLength(3)
  })
})

describe('mapAnalysisScores', () => {
  it('0-1 スコアを百分率に丸める', () => {
    expect(mapAnalysisScores({
      job_score: 0.856,
      interest_score: 0.5,
      aptitude_score: 0,
      future_score: 1,
    })).toEqual({
      job: 86,
      interest: 50,
      aptitude: 0,
      future: 100,
    })
  })
})

describe('buildInterviewQuery', () => {
  it('面接ページ用クエリを組み立てる', () => {
    expect(buildInterviewQuery({
      id: '7',
      name: 'テスト株式会社',
      industry: 'IT',
    })).toBe('company_id=7&company_name=%E3%83%86%E3%82%B9%E3%83%88%E6%A0%AA%E5%BC%8F%E4%BC%9A%E7%A4%BE&industry=IT')
  })
})

describe('collectRelatedCompanyIds / resolveCompanyNameFromRelations', () => {
  const relations: CapitalRelation[] = [
    {
      id: 1,
      parent_id: 10,
      child_id: 20,
      relation_type: 'capital_subsidiary',
      description: '',
      parent: { id: 10, name: '親会社' },
      child: { id: 20, name: '子会社' },
    },
    {
      id: 2,
      from_id: 10,
      to_id: 30,
      relation_type: 'business_partner',
      description: '提携',
      from: { id: 10, name: '親会社' },
      to: { id: 30, name: '取引先' },
    },
  ]

  it('資本関係の関連 ID を収集する', () => {
    expect(Array.from(collectRelatedCompanyIds(10, 'capital', relations)).sort()).toEqual([10, 20])
  })

  it('ビジネス関係の関連 ID を収集する', () => {
    expect(Array.from(collectRelatedCompanyIds(10, 'business', relations)).sort()).toEqual([10, 30])
  })

  it('関係データから企業名を解決する', () => {
    expect(resolveCompanyNameFromRelations(20, relations)).toBe('子会社')
    expect(resolveCompanyNameFromRelations(999, relations)).toBe('企業 999')
  })
})
