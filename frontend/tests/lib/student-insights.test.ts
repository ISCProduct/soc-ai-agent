import {
  displayCategories,
  lowMatchApplications,
  displayIndustries,
  displayTypeLabel,
  formatScore,
  NO_DATA_LABEL,
  type StudentTendency,
} from '@/lib/student-insights'

const withData: StudentTendency = {
  user_id: 1,
  name: '山田太郎',
  email: 'yamada@example.com',
  type_label: '技術探究タイプ',
  top_categories: [
    { category: '技術志向', score: 90 },
    { category: '成長志向', score: 72 },
    { category: '協調性', score: 61 },
    { category: '安定志向', score: 40 },
  ],
  suited_industries: [
    { industry_id: 2, industry_name: 'ソフトウェア開発', score: 81.2 },
    { industry_id: 1, industry_name: '情報通信業', score: 76.4 },
    { industry_id: 5, industry_name: '製造業', score: 60 },
    { industry_id: 9, industry_name: '小売業', score: 42.1 },
  ],
  data_available: true,
}

const withoutData: StudentTendency = {
  user_id: 2,
  name: '佐藤花子',
  email: 'sato@example.com',
  type_label: '分析データ不足',
  top_categories: null,
  suited_industries: null,
  data_available: false,
}

describe('student-insights の表示ロジック', () => {
  describe('displayTypeLabel', () => {
    it('スコアがある生徒はタイプ名を返す', () => {
      expect(displayTypeLabel(withData)).toBe('技術探究タイプ')
    })

    it('data_available が false ならタイプ名を出さない', () => {
      expect(displayTypeLabel(withoutData)).toBe(NO_DATA_LABEL)
    })

    it('data_available が false ならタイプ名が入っていても出さない', () => {
      const spoofed = { ...withoutData, type_label: '技術探究タイプ' }
      expect(displayTypeLabel(spoofed)).toBe(NO_DATA_LABEL)
      expect(displayTypeLabel(spoofed)).not.toBe('技術探究タイプ')
    })

    it('タイプ名が空文字なら分析データ不足として扱う', () => {
      expect(displayTypeLabel({ ...withData, type_label: '' })).toBe(NO_DATA_LABEL)
    })
  })

  describe('displayCategories', () => {
    it('上位3件までに絞る', () => {
      expect(displayCategories(withData).map((c) => c.category)).toEqual([
        '技術志向', '成長志向', '協調性',
      ])
    })

    it('data_available が false なら空配列', () => {
      expect(displayCategories({ ...withoutData, top_categories: withData.top_categories })).toEqual([])
    })

    it('null でも壊れない', () => {
      expect(displayCategories(withoutData)).toEqual([])
    })
  })

  describe('displayIndustries', () => {
    it('TOP3のみ返す', () => {
      expect(displayIndustries(withData).map((i) => i.industry_name)).toEqual([
        'ソフトウェア開発', '情報通信業', '製造業',
      ])
    })

    it('data_available が false なら空配列', () => {
      expect(displayIndustries({ ...withoutData, suited_industries: withData.suited_industries })).toEqual([])
    })

    it('null でも壊れない', () => {
      expect(displayIndustries(withoutData)).toEqual([])
    })
  })

  describe('formatScore', () => {
    it('整数はそのまま、小数は小数第1位まで', () => {
      expect(formatScore(90)).toBe('90')
      expect(formatScore(81.2)).toBe('81.2')
      expect(formatScore(76.44)).toBe('76.4')
    })
  })
})

// バックエンドは該当なしのとき項目ごと省略する(omitempty)ので、
// null と undefined の両方を空として扱えないと画面が落ちる。
describe('lowMatchApplications', () => {
  it.each([
    ['項目なし', undefined],
    ['null', null],
    ['空配列', []],
  ])('%s は空配列を返す', (_name, value) => {
    const s = { ...withData, low_match_applications: value } as StudentTendency
    expect(lowMatchApplications(s)).toEqual([])
  })

  it('応募があればそのまま返す', () => {
    const apps = [{ company_name: '株式会社テスト', match_score: 21.4, status: 'applied' }]
    const s = { ...withData, low_match_applications: apps } as StudentTendency
    expect(lowMatchApplications(s)).toEqual(apps)
  })
})
