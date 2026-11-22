/** @jest-environment jsdom */

import { companyAuthService } from '@/lib/company/auth'

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

  // トークンは httpOnly Cookie にだけ置き、画面は持たない（SOCAIAGENT-683）。
  // X-Company-User-Token は middleware が Cookie から付ける。
  it('ユーザー情報は sessionStorage から読み、トークンはヘッダーに付けない', () => {
    window.sessionStorage.setItem('company_user', JSON.stringify({
      company_user_id: 1,
      company_id: 10,
      email: 'hr@example.com',
      name: '担当',
      role: 'owner',
    }))

    expect(companyAuthService.getStoredUser()?.email).toBe('hr@example.com')
    expect(companyAuthService.getAuthHeaders()).toEqual({})
  })

  it('ログイン後もトークンを sessionStorage に保存しない', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ company_user_id: 1, company_id: 10, email: 'hr@example.com', name: '担当', role: 'owner', token: 'jwt', refresh_token: 'r' }),
    }) as unknown as typeof fetch
    await companyAuthService.login('hr@example.com', 'pw')
    expect(window.sessionStorage.getItem('company_token')).toBeNull()
    expect(companyAuthService.getStoredUser()?.company_user_id).toBe(1)
  })
})
