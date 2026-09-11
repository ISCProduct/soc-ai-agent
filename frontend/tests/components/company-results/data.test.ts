import { generateITCompanies, getBaseCompanyData } from '@/components/company-results/data'
import type { UserData } from '@/components/company-results/types'

describe('getBaseCompanyData', () => {
  it('10社の基本データを返す', () => {
    const companies = getBaseCompanyData()
    expect(companies).toHaveLength(10)
    expect(companies.every(c => c.matchScore === 0)).toBe(true)
    expect(companies[0].name).toBe('株式会社テックイノベーション')
  })
})

describe('generateITCompanies', () => {
  it('最大10社に絞り込む', () => {
    const result = generateITCompanies({})
    expect(result.length).toBeLessThanOrEqual(10)
  })

  it('マッチ度の降順でソートする', () => {
    const userData: UserData = {
      scores: [
        { category: '技術志向', score: 9, reason: 'test' },
        { category: 'チームワーク', score: 8, reason: 'test' },
      ],
    }
    const result = generateITCompanies(userData)

    for (let i = 1; i < result.length; i++) {
      expect(result[i - 1].matchScore).toBeGreaterThanOrEqual(result[i].matchScore)
    }
  })

  it('マッチ度は99を上限とする', () => {
    const userData: UserData = {
      scores: [
        { category: '技術志向', score: 10, reason: 'test' },
        { category: 'コミュニケーション能力', score: 10, reason: 'test' },
        { category: 'リーダーシップ', score: 10, reason: 'test' },
        { category: 'チームワーク', score: 10, reason: 'test' },
        { category: '問題解決力', score: 10, reason: 'test' },
        { category: '創造性・発想力', score: 10, reason: 'test' },
        { category: '計画性・実行力', score: 10, reason: 'test' },
        { category: '学習意欲・成長志向', score: 10, reason: 'test' },
        { category: 'ビジネス思考・目標志向', score: 10, reason: 'test' },
      ],
    }
    const result = generateITCompanies(userData)
    expect(result.every(c => c.matchScore <= 99)).toBe(true)
  })

  it('スコア未指定時はベーススコア50（ストレス耐性未設定時のWLB加点を除く）', () => {
    const result = generateITCompanies({})
    const withoutWlbBonus = result.filter(c => !c.tags.includes('ワークライフバランス'))

    expect(withoutWlbBonus.every(c => c.matchScore === 50)).toBe(true)
    expect(result.find(c => c.tags.includes('ワークライフバランス'))?.matchScore).toBe(60)
  })

  it('技術志向スコアが高いと技術力重視タグの企業が上位に来やすい', () => {
    const userData: UserData = {
      scores: [{ category: '技術志向', score: 10, reason: 'test' }],
    }
    const result = generateITCompanies(userData)
    const techInnovation = result.find(c => c.name === '株式会社テックイノベーション')

    expect(techInnovation).toBeDefined()
    expect(techInnovation!.matchScore).toBeGreaterThan(50)
    expect(result[0].matchScore).toBeGreaterThanOrEqual(techInnovation!.matchScore)
  })

  it('ストレス耐性が低いとワークライフバランスタグの企業に加点される', () => {
    const userData: UserData = {
      scores: [{ category: 'ストレス耐性・粘り強さ', score: 2, reason: 'test' }],
    }
    const result = generateITCompanies(userData)
    const enterprise = result.find(c => c.name === 'エンタープライズシステムズ株式会社')

    expect(enterprise).toBeDefined()
    expect(enterprise!.matchScore).toBe(60)
  })

  it('userData.scores からカテゴリ別スコアを反映する', () => {
    const userData: UserData = {
      scores: [{ category: '計画性・実行力', score: 8, reason: 'test' }],
    }
    const result = generateITCompanies(userData)
    const sier = result.find(c => c.industry.includes('SIer'))

    expect(sier).toBeDefined()
    expect(sier!.matchScore).toBeGreaterThan(50)
  })
})
