/** @jest-environment jsdom */

import { companyAuthService } from '@/lib/company-auth'

function createStorageMock(initial: Record<string, string> = {}): Storage {
  const store = new Map<string, string>(Object.entries(initial))
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
}

describe('companyAuthService', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'sessionStorage', {
      value: createStorageMock(),
      configurable: true,
    })
  })

  it('stores auth headers from sessionStorage', () => {
    window.sessionStorage.setItem('company_user', JSON.stringify({
      company_user_id: 1,
      company_id: 10,
      email: 'hr@example.com',
      name: '担当',
      role: 'owner',
    }))
    window.sessionStorage.setItem('company_token', 'jwt-token')

    expect(companyAuthService.getStoredUser()?.email).toBe('hr@example.com')
    expect(companyAuthService.getStoredToken()).toBe('jwt-token')
    // X-Company-User-ID は Backend の EchoCompanyAuth では参照されず、
    // なりすまし防止のため httpOnly Cookie 経由でのみ注入する(クライアントJSからは送らない)
    expect(companyAuthService.getAuthHeaders()).toEqual({
      'X-Company-User-Token': 'jwt-token',
    })
  })

  it('returns empty headers when no token is stored', () => {
    expect(companyAuthService.getAuthHeaders()).toEqual({})
  })
})
