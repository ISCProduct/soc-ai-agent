import {
  parseCompanyId,
  buildCompanyDetailPath,
  getCompanyIdFromNode,
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

  describe('getCompanyIdFromNode', () => {
    it('prefers data.companyId over node.id', () => {
      expect(
        getCompanyIdFromNode({
          id: '99',
          data: { companyId: 15 },
        })
      ).toBe(15)
    })

    it('falls back to node.id', () => {
      expect(getCompanyIdFromNode({ id: '8' })).toBe(8)
    })

    it('returns null for invalid ids', () => {
      expect(getCompanyIdFromNode({ id: 'invalid' })).toBeNull()
      expect(getCompanyIdFromNode({ id: '0' })).toBeNull()
    })
  })

  describe('getCompanyDetailPathFromNode', () => {
    it('builds path from node company id', () => {
      expect(
        getCompanyDetailPathFromNode({
          id: '99',
          data: { companyId: 15 },
        })
      ).toBe('/company/15')
    })

    it('returns null when id is invalid', () => {
      expect(
        getCompanyDetailPathFromNode({
          id: 'invalid',
          data: { companyId: 'x' },
        })
      ).toBeNull()
    })
  })
})
