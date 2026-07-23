import {
  parseCompanyId,
  buildCompanyDetailPath,
  getCompanyDetailPathFromNode,
} from '@/lib/correlation-diagram-navigation'

describe('correlation-diagram-navigation', () => {
  describe('parseCompanyId', () => {
    it('accepts positive integers', () => {
      expect(parseCompanyId(1)).toBe(1)
      expect(parseCompanyId(42)).toBe(42)
    })

    it('accepts numeric strings', () => {
      expect(parseCompanyId('7')).toBe(7)
      expect(parseCompanyId(' 12 ')).toBe(12)
    })

    it('rejects zero, negative, float, empty, and non-numeric', () => {
      expect(parseCompanyId(0)).toBeNull()
      expect(parseCompanyId(-1)).toBeNull()
      expect(parseCompanyId(1.5)).toBeNull()
      expect(parseCompanyId('')).toBeNull()
      expect(parseCompanyId('  ')).toBeNull()
      expect(parseCompanyId('abc')).toBeNull()
      expect(parseCompanyId(null)).toBeNull()
      expect(parseCompanyId(undefined)).toBeNull()
      expect(parseCompanyId({})).toBeNull()
    })
  })

  describe('buildCompanyDetailPath', () => {
    it('builds /company/:id path', () => {
      expect(buildCompanyDetailPath(15)).toBe('/company/15')
    })
  })

  describe('getCompanyDetailPathFromNode', () => {
    it('prefers data.companyId over node.id', () => {
      expect(
        getCompanyDetailPathFromNode({
          id: '99',
          data: { companyId: 15 },
        })
      ).toBe('/company/15')
    })

    it('falls back to node.id when data.companyId is missing', () => {
      expect(getCompanyDetailPathFromNode({ id: '8' })).toBe('/company/8')
    })

    it('returns null when both id sources are invalid', () => {
      expect(
        getCompanyDetailPathFromNode({
          id: 'invalid',
          data: { companyId: 'x' },
        })
      ).toBeNull()
      expect(getCompanyDetailPathFromNode({ id: '0' })).toBeNull()
    })
  })
})
