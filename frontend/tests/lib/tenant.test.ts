import { extractTenantSlug, isAdminHost } from '@/lib/tenant'

describe('tenant', () => {
  describe('extractTenantSlug', () => {
    it('学園サブドメインを抽出する', () => {
      expect(extractTenantSlug('my-school.shukatsu-ai.jp')).toBe('my-school')
      expect(extractTenantSlug('my-school.shukatsu-ai.jp:443')).toBe('my-school')
    })

    it('非テナントホストではundefinedを返す', () => {
      expect(extractTenantSlug('shukatsu-ai.jp')).toBeUndefined()
      expect(extractTenantSlug('www.shukatsu-ai.jp')).toBeUndefined()
      expect(extractTenantSlug('api.shukatsu-ai.jp')).toBeUndefined()
      expect(extractTenantSlug('admin.shukatsu-ai.jp')).toBeUndefined()
      expect(extractTenantSlug('localhost:3000')).toBeUndefined()
    })
  })

  describe('isAdminHost', () => {
    it('adminサブドメインを判定する', () => {
      expect(isAdminHost('admin.shukatsu-ai.jp')).toBe(true)
      expect(isAdminHost('admin.shukatsu-ai.jp:443')).toBe(true)
    })

    it('adminサブドメイン以外はfalse', () => {
      expect(isAdminHost('shukatsu-ai.jp')).toBe(false)
      expect(isAdminHost('my-school.shukatsu-ai.jp')).toBe(false)
    })

    it('紛らわしい類似ホストを誤って管理画面扱いしない', () => {
      expect(isAdminHost('admin.example.com')).toBe(false)
      expect(isAdminHost('admin.foo.shukatsu-ai.jp')).toBe(false)
      expect(isAdminHost('administrator.shukatsu-ai.jp')).toBe(false)
    })
  })
})
