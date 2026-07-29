import {
  infoFieldEnabled,
  resolveIndustryFieldProfile,
  techFieldSpec,
} from '@/lib/admin-company-field-profile'

describe('admin-company-field-profile', () => {
  describe('resolveIndustryFieldProfile', () => {
    it('resolves IT industries and requires tech for publish', () => {
      const profile = resolveIndustryFieldProfile('IT・ソフトウェア')
      expect(profile.id).toBe('it')
      expect(profile.techAspectEnabled).toBe(true)
      expect(profile.requireTechForPublish).toBe(true)
      expect(profile.techAspectLabel).toBe('技術情報')
      expect(techFieldSpec(profile, 'cicd_tools')).toBeDefined()
    })

    it('resolves manufacturing with optional equipment fields', () => {
      const profile = resolveIndustryFieldProfile('自動車製造')
      expect(profile.id).toBe('manufacturing')
      expect(profile.techAspectEnabled).toBe(true)
      expect(profile.requireTechForPublish).toBe(false)
      expect(profile.techAspectLabel).toBe('設備・技術')
      expect(techFieldSpec(profile, 'cicd_tools')).toBeUndefined()
      expect(techFieldSpec(profile, 'infra_stack')?.label).toContain('生産設備')
    })

    it('hides tech aspect for finance/education/healthcare', () => {
      for (const industry of ['金融・保険業', '教育・学習支援業', '医療・福祉', 'コンサルティング']) {
        const profile = resolveIndustryFieldProfile(industry)
        expect(profile.techAspectEnabled).toBe(false)
        expect(profile.requireTechForPublish).toBe(false)
        expect(profile.techFields).toHaveLength(0)
      }
    })

    it('falls back to general for unknown or empty industry', () => {
      expect(resolveIndustryFieldProfile('').id).toBe('general')
      expect(resolveIndustryFieldProfile('小売業').id).toBe('general')
      expect(resolveIndustryFieldProfile(null).techAspectEnabled).toBe(false)
    })
  })

  describe('infoFieldEnabled', () => {
    it('does not expose tech_stack on info form by default', () => {
      const profile = resolveIndustryFieldProfile('IT・ソフトウェア')
      expect(infoFieldEnabled(profile, 'description')).toBe(true)
      expect(infoFieldEnabled(profile, 'industry')).toBe(true)
      expect(infoFieldEnabled(profile, 'tech_stack')).toBe(false)
    })
  })
})
