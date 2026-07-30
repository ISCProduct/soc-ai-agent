import {
  formatFetchPrimaryEmptyAspects,
  formatFetchPrimarySummary,
  hasActionableSoftEmpty,
  isActionableEmptyStep,
  type FetchPrimaryResponse,
} from '@/lib/admin-company-fetch'

describe('admin-company-fetch', () => {
  const base: FetchPrimaryResponse = {
    info_step: { status: 'fetched', detail: 'ok' },
    tech_step: { status: 'empty', detail: 'no_tech_stack' },
    relations_step: { status: 'fetched', detail: 'ok', count: 2 },
  }

  describe('isActionableEmptyStep / hasActionableSoftEmpty', () => {
    it('treats tech empty as actionable for IT', () => {
      expect(isActionableEmptyStep(base.tech_step, 'tech', 'IT・ソフトウェア')).toBe(true)
      expect(hasActionableSoftEmpty(base, 'IT・ソフトウェア')).toBe(true)
    })

    it('ignores tech empty for manufacturing and finance', () => {
      expect(isActionableEmptyStep(base.tech_step, 'tech', '製造業')).toBe(false)
      expect(hasActionableSoftEmpty(base, '製造業')).toBe(false)
      expect(hasActionableSoftEmpty(base, '金融・保険業')).toBe(false)
    })

    it('uses company.industry from response when fallback is empty', () => {
      const data: FetchPrimaryResponse = {
        ...base,
        company: { industry: '製造業' },
      }
      expect(hasActionableSoftEmpty(data, '')).toBe(false)
    })

    it('still warns when info is empty regardless of industry', () => {
      const data: FetchPrimaryResponse = {
        info_step: { status: 'empty', detail: 'no_basic_info' },
        tech_step: { status: 'skipped', detail: 'industry_not_applicable' },
        relations_step: { status: 'fetched' },
      }
      expect(hasActionableSoftEmpty(data, '金融・保険業')).toBe(true)
    })

    it('treats public_info_sparse as soft-warn info empty', () => {
      const data: FetchPrimaryResponse = {
        info_step: { status: 'empty', detail: 'public_info_sparse' },
        tech_step: { status: 'skipped', detail: 'industry_not_applicable' },
        relations_step: { status: 'fetched', detail: 'confirmed_unlisted', count: 1 },
      }
      expect(hasActionableSoftEmpty(data, '金融・保険業')).toBe(true)
      expect(formatFetchPrimaryEmptyAspects(data, '金融・保険業')).toContain('公開情報から未特定')
    })
  })

  describe('formatFetchPrimarySummary', () => {
    it('labels tech aspect by industry profile', () => {
      const itSummary = formatFetchPrimarySummary(base, 'IT・ソフトウェア')
      expect(itSummary).toContain('基本取得')
      expect(itSummary).toContain('技術情報取得ゼロ')
      expect(itSummary).toContain('関係取得(2)')

      const mfgSummary = formatFetchPrimarySummary(
        {
          ...base,
          tech_step: { status: 'skipped', detail: 'optional_empty' },
        },
        '製造業',
      )
      expect(mfgSummary).toContain('設備・技術スキップ(任意・未特定)')
    })

    it('explains industry_not_applicable skip reason', () => {
      const summary = formatFetchPrimarySummary(
        {
          info_step: { status: 'fetched' },
          tech_step: { status: 'skipped', detail: 'industry_not_applicable' },
          relations_step: { status: 'fetched' },
        },
        '金融・保険業',
      )
      expect(summary).toContain('技術情報スキップ(業種対象外)')
    })

    it('labels confirmed sparse / unlisted outcomes', () => {
      const summary = formatFetchPrimarySummary(
        {
          info_step: { status: 'empty', detail: 'public_info_sparse' },
          tech_step: { status: 'skipped', detail: 'industry_not_applicable' },
          relations_step: { status: 'fetched', detail: 'confirmed_unlisted', count: 1 },
        },
        '金融・保険業',
      )
      expect(summary).toContain('基本確認済(概要未特定)')
      expect(summary).toContain('関係確認済(非上場・関係なし)')
    })
  })

  describe('formatFetchPrimaryEmptyAspects', () => {
    it('lists only actionable empty aspects', () => {
      expect(formatFetchPrimaryEmptyAspects(base, 'IT・ソフトウェア')).toBe('技術情報')
      expect(formatFetchPrimaryEmptyAspects(base, '製造業')).toBe('')
      expect(
        formatFetchPrimaryEmptyAspects(
          {
            info_step: { status: 'empty' },
            tech_step: { status: 'empty' },
            relations_step: { status: 'empty' },
          },
          'IT・ソフトウェア',
        ),
      ).toBe('会社概要・技術情報・関連企業')
    })
  })
})
